package proof

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Watermark = `":"`

type CheckSpec struct {
	FilesExist    []string          `json:"files_exist,omitempty"`
	FilesChanged  []string          `json:"files_changed,omitempty"`
	Commands      []string          `json:"commands,omitempty"`
	HTTP          []string          `json:"http,omitempty"`
	GitClean      *bool             `json:"git_clean,omitempty"`
	GitCommit     bool              `json:"git_commit,omitempty"`
	EnvEquals     map[string]string `json:"env_equals,omitempty"`
	TimeoutSecond int               `json:"timeout_seconds,omitempty"`
}

type RunSpec struct {
	Name           string    `json:"name"`
	Command        []string  `json:"command"`
	Workdir        string    `json:"workdir,omitempty"`
	TimeoutSeconds int       `json:"timeout_seconds,omitempty"`
	RecoverCommand []string  `json:"recover_command,omitempty"`
	Checks         CheckSpec `json:"checks"`
}

type CheckResult struct {
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Passed   bool   `json:"passed"`
	Observed string `json:"observed,omitempty"`
	Error    string `json:"error,omitempty"`
}

type ProcessResult struct {
	Command     []string `json:"command"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at"`
	DurationMS  int64    `json:"duration_ms"`
	ExitCode    int      `json:"exit_code"`
	TimedOut    bool     `json:"timed_out"`
	Stdout      string   `json:"stdout"`
	Stderr      string   `json:"stderr"`
	ProcessID   int      `json:"process_id,omitempty"`
	Interrupted bool     `json:"interrupted"`
}

type RecoveryResult struct {
	Attempted bool          `json:"attempted"`
	Reason    string        `json:"reason,omitempty"`
	Process   ProcessResult `json:"process,omitempty"`
}

type Proof struct {
	SchemaVersion  string         `json:"schema_version"`
	Watermark      string         `json:"watermark"`
	RunID          string         `json:"run_id"`
	Name           string         `json:"name"`
	Host           HostInfo       `json:"host"`
	Request        RunSpec        `json:"request"`
	Initial        ProcessResult  `json:"initial"`
	ChecksBefore   []CheckResult  `json:"checks_before_recovery"`
	Recovery       RecoveryResult `json:"recovery"`
	ChecksAfter    []CheckResult  `json:"checks_after_recovery"`
	Classification string         `json:"classification"`
	Result         string         `json:"result"`
	CreatedAt      string         `json:"created_at"`
	EvidenceHash   string         `json:"evidence_sha256"`
}

type HostInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
}

type Snapshot map[string]string

func Host() HostInfo {
	h, _ := os.Hostname()
	return HostInfo{OS: runtime.GOOS, Arch: runtime.GOARCH, Hostname: h}
}

func NowID() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z")
}

func SnapshotFiles(paths []string, wd string) Snapshot {
	out := Snapshot{}
	for _, p := range paths {
		rp := resolve(wd, p)
		out[p] = hashFileOrState(rp)
	}
	return out
}

func hashFileOrState(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "ABSENT"
		}
		return "ERROR:" + err.Error()
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func resolve(wd, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if wd == "" {
		wd = "."
	}
	return filepath.Join(wd, p)
}

func Run(ctx context.Context, cmd []string, wd string, timeout time.Duration, stdoutPath, stderrPath string) ProcessResult {
	res := ProcessResult{Command: append([]string{}, cmd...), StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if len(cmd) == 0 {
		res.ExitCode = 127
		res.Stderr = "empty command"
		res.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return res
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	if wd != "" {
		c.Dir = wd
	}
	if runtime.GOOS != "windows" {
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	var outBuf, errBuf bytes.Buffer
	outWriters := []io.Writer{&outBuf}
	errWriters := []io.Writer{&errBuf}
	if stdoutPath != "" {
		f, err := os.Create(stdoutPath)
		if err == nil {
			defer f.Close()
			outWriters = append(outWriters, f)
		}
	}
	if stderrPath != "" {
		f, err := os.Create(stderrPath)
		if err == nil {
			defer f.Close()
			errWriters = append(errWriters, f)
		}
	}
	c.Stdout = io.MultiWriter(outWriters...)
	c.Stderr = io.MultiWriter(errWriters...)

	start := time.Now()
	if err := c.Start(); err != nil {
		res.ExitCode = 127
		res.Stderr = err.Error()
		res.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	res.ProcessID = c.Process.Pid

	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		res.Interrupted = true
		killProcessTree(c)
		err = <-done
	case <-timer.C:
		res.TimedOut = true
		res.Interrupted = true
		killProcessTree(c)
		err = <-done
	}

	res.DurationMS = time.Since(start).Milliseconds()
	res.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	res.Stdout = outBuf.String()
	res.Stderr = errBuf.String()
	if res.TimedOut {
		res.ExitCode = 124
		return res
	}
	if ctx.Err() != nil {
		res.ExitCode = 130
		return res
	}
	if err == nil {
		res.ExitCode = 0
		return res
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.ExitCode = ee.ExitCode()
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			res.Interrupted = true
		}
		return res
	}
	res.ExitCode = 1
	return res
}

func killProcessTree(c *exec.Cmd) {
	if c == nil || c.Process == nil {
		return
	}
	if runtime.GOOS != "windows" {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		return
	}
	_ = c.Process.Kill()
}

func Verify(ctx context.Context, spec RunSpec, before Snapshot) []CheckResult {
	wd := spec.Workdir
	results := make([]CheckResult, 0)

	for _, p := range spec.Checks.FilesExist {
		rp := resolve(wd, p)
		_, err := os.Stat(rp)
		r := CheckResult{Kind: "file_exists", Target: p, Passed: err == nil}
		if err != nil {
			r.Error = err.Error()
		} else {
			r.Observed = "present"
		}
		results = append(results, r)
	}

	for _, p := range spec.Checks.FilesChanged {
		rp := resolve(wd, p)
		after := hashFileOrState(rp)
		prev := before[p]
		pass := after != prev && !strings.HasPrefix(after, "ERROR:")
		r := CheckResult{Kind: "file_changed", Target: p, Passed: pass, Observed: "before=" + prev + " after=" + after}
		results = append(results, r)
	}

	for _, c := range spec.Checks.Commands {
		timeout := 60 * time.Second
		if spec.Checks.TimeoutSecond > 0 {
			timeout = time.Duration(spec.Checks.TimeoutSecond) * time.Second
		}
		pr := Run(ctx, []string{"sh", "-lc", c}, wd, timeout, "", "")
		r := CheckResult{
			Kind:     "command",
			Target:   c,
			Passed:   pr.ExitCode == 0 && !pr.TimedOut,
			Observed: strings.TrimSpace(pr.Stdout),
		}
		if !r.Passed {
			r.Error = strings.TrimSpace(pr.Stderr)
			if pr.TimedOut {
				r.Error = "timed out"
			}
		}
		results = append(results, r)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, u := range spec.Checks.HTTP {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		resp, err := client.Do(req)
		r := CheckResult{Kind: "http", Target: u}
		if err != nil {
			r.Error = err.Error()
		} else {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			r.Observed = strconv.Itoa(resp.StatusCode)
			r.Passed = resp.StatusCode >= 200 && resp.StatusCode < 400
			if !r.Passed {
				r.Error = resp.Status
			}
		}
		results = append(results, r)
	}

	if spec.Checks.GitClean != nil {
		pr := Run(ctx, []string{"git", "status", "--porcelain"}, wd, 20*time.Second, "", "")
		clean := strings.TrimSpace(pr.Stdout) == ""
		expected := *spec.Checks.GitClean
		results = append(results, CheckResult{
			Kind: "git_clean", Target: wd, Passed: pr.ExitCode == 0 && clean == expected,
			Observed: fmt.Sprintf("exit=%d clean=%v expected=%v", pr.ExitCode, clean, expected),
			Error: strings.TrimSpace(pr.Stderr),
		})
	}

	if spec.Checks.GitCommit {
		pr := Run(ctx, []string{"git", "rev-parse", "--verify", "HEAD"}, wd, 20*time.Second, "", "")
		results = append(results, CheckResult{
			Kind: "git_commit", Target: "HEAD", Passed: pr.ExitCode == 0,
			Observed: strings.TrimSpace(pr.Stdout), Error: strings.TrimSpace(pr.Stderr),
		})
	}

	keys := make([]string, 0, len(spec.Checks.EnvEquals))
	for k := range spec.Checks.EnvEquals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		exp := spec.Checks.EnvEquals[k]
		got := os.Getenv(k)
		results = append(results, CheckResult{
			Kind: "env_equals", Target: k, Passed: got == exp,
			Observed: got, Error: mismatch(got, exp),
		})
	}

	return results
}

func mismatch(got, exp string) string {
	if got == exp {
		return ""
	}
	return fmt.Sprintf("got %q expected %q", got, exp)
}

func AllPassed(rs []CheckResult) bool {
	if len(rs) == 0 {
		return false
	}
	for _, r := range rs {
		if !r.Passed {
			return false
		}
	}
	return true
}

func Classify(proc ProcessResult, checks []CheckResult) string {
	if proc.TimedOut {
		return "HUNG_OR_TIMEOUT"
	}
	if proc.ExitCode != 0 {
		return "EXECUTION_FAILED"
	}
	if len(checks) == 0 {
		return "UNVERIFIED_NO_CHECKS"
	}
	if !AllPassed(checks) {
		return "FALSE_SUCCESS_OR_PARTIAL_EXECUTION"
	}
	return "VERIFIED_SUCCESS"
}

func EvidenceHash(p Proof) string {
	cp := p
	cp.EvidenceHash = ""
	b, _ := json.Marshal(cp)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func SaveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func LoadSpec(path string) (RunSpec, error) {
	var s RunSpec
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	if err != nil {
		return s, err
	}
	if len(s.Command) == 0 {
		return s, errors.New("spec.command cannot be empty")
	}
	if s.Name == "" {
		s.Name = "agent-proof-run"
	}
	return s, nil
}

func WriteCheckpoint(dir, state string, payload any) error {
	cpDir := filepath.Join(dir, "checkpoints")
	if err := os.MkdirAll(cpDir, 0o755); err != nil {
		return err
	}
	f := filepath.Join(cpDir, time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+state+".json")
	return SaveJSON(f, map[string]any{
		"state": state, "at": time.Now().UTC().Format(time.RFC3339Nano), "payload": payload, "watermark": Watermark,
	})
}

type StreamEvent struct {
	At      string `json:"at"`
	Stream  string `json:"stream"`
	Message string `json:"message"`
}

func CopyLinesToJSONL(dst string, stream string, src string) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(strings.NewReader(src))
	enc := json.NewEncoder(f)
	for s.Scan() {
		if err := enc.Encode(StreamEvent{At: time.Now().UTC().Format(time.RFC3339Nano), Stream: stream, Message: s.Text()}); err != nil {
			return err
		}
	}
	return s.Err()
}

func RunProof(ctx context.Context, spec RunSpec, root string) (Proof, string, error) {
	runID := NowID()
	dir := filepath.Join(root, runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Proof{}, "", err
	}

	if spec.Workdir == "" {
		spec.Workdir = "."
	}
	absWD, err := filepath.Abs(spec.Workdir)
	if err == nil {
		spec.Workdir = absWD
	}

	before := SnapshotFiles(spec.Checks.FilesChanged, spec.Workdir)
	_ = SaveJSON(filepath.Join(dir, "request.json"), spec)
	_ = SaveJSON(filepath.Join(dir, "snapshot-before.json"), before)
	_ = WriteCheckpoint(dir, "before-execution", map[string]any{"snapshot": before})

	timeout := 300 * time.Second
	if spec.TimeoutSeconds > 0 {
		timeout = time.Duration(spec.TimeoutSeconds) * time.Second
	}

	initial := Run(ctx, spec.Command, spec.Workdir, timeout, filepath.Join(dir, "stdout.log"), filepath.Join(dir, "stderr.log"))
	_ = SaveJSON(filepath.Join(dir, "process.json"), initial)
	_ = CopyLinesToJSONL(filepath.Join(dir, "execution.jsonl"), "stdout", initial.Stdout)
	_ = CopyLinesToJSONL(filepath.Join(dir, "execution.jsonl"), "stderr", initial.Stderr)
	_ = WriteCheckpoint(dir, "after-execution", initial)

	checksBefore := Verify(ctx, spec, before)
	_ = SaveJSON(filepath.Join(dir, "checks-before.json"), checksBefore)

	class := Classify(initial, checksBefore)
	recovery := RecoveryResult{}
	checksAfter := checksBefore

	if class != "VERIFIED_SUCCESS" && len(spec.RecoverCommand) > 0 {
		recovery.Attempted = true
		recovery.Reason = class
		_ = WriteCheckpoint(dir, "before-recovery", map[string]any{"classification": class})
		recovery.Process = Run(ctx, spec.RecoverCommand, spec.Workdir, timeout, filepath.Join(dir, "recovery-stdout.log"), filepath.Join(dir, "recovery-stderr.log"))
		_ = SaveJSON(filepath.Join(dir, "recovery.json"), recovery)
		checksAfter = Verify(ctx, spec, before)
		_ = SaveJSON(filepath.Join(dir, "checks-after.json"), checksAfter)
		_ = WriteCheckpoint(dir, "after-recovery", map[string]any{"checks": checksAfter})
		if recovery.Process.ExitCode == 0 && AllPassed(checksAfter) {
			class = "RECOVERED_AND_VERIFIED"
		} else {
			class = Classify(recovery.Process, checksAfter)
		}
	}

	result := "FAIL"
	if class == "UNVERIFIED_NO_CHECKS" {
		result = "UNVERIFIED"
	}
	if class == "VERIFIED_SUCCESS" || class == "RECOVERED_AND_VERIFIED" {
		result = "PASS"
	}

	p := Proof{
		SchemaVersion:  "1.0",
		Watermark:      Watermark,
		RunID:          runID,
		Name:           spec.Name,
		Host:           Host(),
		Request:        spec,
		Initial:        initial,
		ChecksBefore:   checksBefore,
		Recovery:       recovery,
		ChecksAfter:    checksAfter,
		Classification: class,
		Result:         result,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	p.EvidenceHash = EvidenceHash(p)
	if err := SaveJSON(filepath.Join(dir, "proof.json"), p); err != nil {
		return p, dir, err
	}

	manifest := map[string]any{
		"run_id": runID, "result": result, "classification": class,
		"evidence_sha256": p.EvidenceHash, "watermark": Watermark,
	}
	_ = SaveJSON(filepath.Join(dir, "manifest.json"), manifest)
	return p, dir, nil
}

func ChaosScenario(name, root string) (Proof, string, error) {
	tmp, err := os.MkdirTemp("", "agent-proof-chaos-*")
	if err != nil {
		return Proof{}, "", err
	}
	defer os.RemoveAll(tmp)

	var spec RunSpec
	switch name {
	case "fake-success":
		spec = RunSpec{
			Name:           "chaos-fake-success",
			Command:        []string{"sh", "-lc", "echo 'done'; exit 0"},
			Workdir:        tmp,
			TimeoutSeconds: 5,
			Checks:         CheckSpec{FilesExist: []string{"expected.artifact"}},
		}
	case "hung-agent":
		spec = RunSpec{
			Name:           "chaos-hung-agent",
			Command:        []string{"sh", "-lc", "sleep 10"},
			Workdir:        tmp,
			TimeoutSeconds: 1,
			Checks:         CheckSpec{Commands: []string{"true"}},
		}
	case "partial-execution":
		spec = RunSpec{
			Name:           "chaos-partial-execution",
			Command:        []string{"sh", "-lc", "printf ok > one.txt; exit 0"},
			Workdir:        tmp,
			TimeoutSeconds: 5,
			Checks:         CheckSpec{FilesExist: []string{"one.txt", "two.txt"}},
		}
	case "recovery":
		spec = RunSpec{
			Name:           "chaos-recovery",
			Command:        []string{"sh", "-lc", "echo 'pretend success'"},
			RecoverCommand: []string{"sh", "-lc", "printf recovered > expected.artifact"},
			Workdir:        tmp,
			TimeoutSeconds: 5,
			Checks:         CheckSpec{FilesExist: []string{"expected.artifact"}},
		}
	case "changed-file":
		os.WriteFile(filepath.Join(tmp, "target.txt"), []byte("before\n"), 0o644)
		spec = RunSpec{
			Name:           "chaos-changed-file",
			Command:        []string{"sh", "-lc", "printf 'after\n' > target.txt"},
			Workdir:        tmp,
			TimeoutSeconds: 5,
			Checks:         CheckSpec{FilesChanged: []string{"target.txt"}},
		}
	default:
		return Proof{}, "", fmt.Errorf("unknown chaos scenario %q", name)
	}
	return RunProof(context.Background(), spec, root)
}

func ScanEvidence(root string) ([]Proof, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := make([]Proof, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name(), "proof.json")
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			var pf Proof
			b, err := os.ReadFile(path)
			if err != nil || json.Unmarshal(b, &pf) != nil {
				return
			}
			mu.Lock()
			out = append(out, pf)
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}
