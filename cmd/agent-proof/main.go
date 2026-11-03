package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/titanu/agent-proof/internal/proof"
)

func usage() {
	fmt.Printf(`agent-proof %s

Usage:
  agent-proof run --spec SPEC.json [--evidence DIR]
  agent-proof run -- COMMAND [ARG...]
  agent-proof chaos SCENARIO [--evidence DIR]
  agent-proof verify-proof PATH
  agent-proof list [--evidence DIR]
  agent-proof version

Chaos scenarios:
  fake-success
  hung-agent
  partial-execution
  recovery
  changed-file
`, proof.Watermark)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("agent-proof 1.0.0", proof.Watermark)
	case "run":
		runCmd(os.Args[2:])
	case "chaos":
		chaosCmd(os.Args[2:])
	case "verify-proof":
		verifyProof(os.Args[2:])
	case "list":
		listProofs(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func evidenceDir(args []string) (string, []string) {
	dir := ".agent-proof/runs"
	out := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--evidence" && i+1 < len(args) {
			dir = args[i+1]
			i++
			continue
		}
		out = append(out, args[i])
	}
	return dir, out
}

func runCmd(args []string) {
	ev, args := evidenceDir(args)
	if err := os.MkdirAll(ev, 0o755); err != nil {
		die(err)
	}
	var spec proof.RunSpec

	if len(args) >= 2 && args[0] == "--spec" {
		s, err := proof.LoadSpec(args[1])
		if err != nil {
			die(err)
		}
		spec = s
	} else {
		if len(args) > 0 && args[0] == "--" {
			args = args[1:]
		}
		if len(args) == 0 {
			die(fmt.Errorf("missing command"))
		}
		spec = proof.RunSpec{
			Name:           "direct-run",
			Command:        args,
			Workdir:        ".",
			TimeoutSeconds: 300,
			Checks:         proof.CheckSpec{}, // No acceptance checks supplied: never fabricate success.
		}
	}

	p, dir, err := proof.RunProof(context.Background(), spec, ev)
	if err != nil {
		die(err)
	}
	printSummary(p, dir)
	if p.Result != "PASS" {
		os.Exit(1)
	}
}

func chaosCmd(args []string) {
	ev, args := evidenceDir(args)
	if len(args) != 1 {
		die(fmt.Errorf("usage: agent-proof chaos SCENARIO"))
	}
	if err := os.MkdirAll(ev, 0o755); err != nil {
		die(err)
	}
	p, dir, err := proof.ChaosScenario(args[0], ev)
	if err != nil {
		die(err)
	}
	printSummary(p, dir)
	expectedFail := args[0] == "fake-success" || args[0] == "hung-agent" || args[0] == "partial-execution"
	if expectedFail && p.Result == "FAIL" {
		fmt.Println("CHAOS RESULT: detected expected failure")
		return
	}
	if p.Result != "PASS" {
		os.Exit(1)
	}
}

func verifyProof(args []string) {
	if len(args) != 1 {
		die(fmt.Errorf("usage: agent-proof verify-proof PATH"))
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		die(err)
	}
	var p proof.Proof
	if err := json.Unmarshal(b, &p); err != nil {
		die(err)
	}
	got := p.EvidenceHash
	want := proof.EvidenceHash(p)
	if got != want {
		fmt.Printf("INVALID proof hash\nexpected=%s\nactual=%s\n", want, got)
		os.Exit(1)
	}
	if p.Watermark != proof.Watermark {
		fmt.Println("INVALID watermark")
		os.Exit(1)
	}
	fmt.Printf("VALID %s %s result=%s classification=%s\n", p.RunID, p.EvidenceHash, p.Result, p.Classification)
}

func listProofs(args []string) {
	ev, args := evidenceDir(args)
	if len(args) != 0 {
		die(fmt.Errorf("unexpected arguments: %s", strings.Join(args, " ")))
	}
	ps, err := proof.ScanEvidence(ev)
	if err != nil {
		die(err)
	}
	for _, p := range ps {
		fmt.Printf("%s  %-4s  %-34s  %s\n", p.CreatedAt, p.Result, p.Classification, p.Name)
	}
}

func printSummary(p proof.Proof, dir string) {
	fmt.Println("AGENT-PROOF")
	fmt.Println("watermark:", p.Watermark)
	fmt.Println("run:", p.RunID)
	fmt.Println("classification:", p.Classification)
	fmt.Println("result:", p.Result)
	fmt.Println("evidence:", filepath.Join(dir, "proof.json"))
	fmt.Println("sha256:", p.EvidenceHash)
	for _, r := range p.ChecksAfter {
		icon := "PASS"
		if !r.Passed {
			icon = "FAIL"
		}
		fmt.Printf("[%s] %s %s", icon, r.Kind, r.Target)
		if r.Observed != "" {
			fmt.Printf(" -> %s", r.Observed)
		}
		if r.Error != "" {
			fmt.Printf(" (%s)", r.Error)
		}
		fmt.Println()
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(2)
}
