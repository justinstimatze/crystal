// Package cmdverify is the operational, golden-free verifier of the dogfood
// harness — the keepable tooling. Given a generated kong command struct, it
// compiles the scaffold in isolation and probes it through a LAYERED property
// gate, each layer extending coverage:
//
//	builds    — go build of a standalone kong program embedding the struct
//	registers — the built binary exposes the verb (`bin <verb> --help` exits 0)
//	contract  — a behavioral probe passes (e.g. an enum flag rejects an
//	            off-list value; a required flag is accepted)
//
// The Go compiler and kong ARE the verifier: deterministic, no model, no
// pre-known golden. It is strictly stronger than golden-equality because it
// certifies a property, not a memorized string — but it is structural, so a
// behaviorally-wrong-yet-compiling scaffold can leak past `builds`/`registers`.
// Measuring exactly where that leak begins is the point of the harness.
//
// The scratch program is written under a subdir of the CURRENT module so it
// resolves kong from the repo's go.mod/go.sum (offline, no scratch module).
package cmdverify

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Contract is the optional behavioral probe for a command's shape class.
type Contract struct {
	EnumFlag   string // kebab CLI flag that must reject OffListVal (exit != 0)
	OffListVal string
	NeedFlag   string // kebab CLI flag that must be accepted (exit == 0)
}

// Verdict is the layered outcome. Contract is meaningful only if HasContract.
type Verdict struct {
	Builds      bool
	Registers   bool
	Contract    bool
	HasContract bool
	Detail      string
}

// Pass reports whether every applicable layer passed.
func (v Verdict) Pass() bool {
	if !v.Builds || !v.Registers {
		return false
	}
	return !v.HasContract || v.Contract
}

const mainTmpl = `package main

import "github.com/alecthomas/kong"

%s

type dogfoodRoot struct {
	%s %s ` + "`cmd:\"\" name:\"%s\" help:\"generated\"`" + `
}

func main() { _ = kong.Parse(&dogfoodRoot{}) }
`

// Verify compiles structSrc (a `type <structName> struct { ... }` declaration)
// as a standalone kong command under scratchRoot/<verb> and probes it. verb is
// the kong CLI name; field is the exported root field (structName minus "Cmd").
func Verify(scratchRoot, structName, field, verb, structSrc string, c *Contract) (Verdict, error) {
	dir := filepath.Join(scratchRoot, verb)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Verdict{}, err
	}
	src := []byte(fmt.Sprintf(mainTmpl, structSrc, field, structName, verb))
	if err := os.WriteFile(filepath.Join(dir, "main.go"), src, 0o644); err != nil {
		return Verdict{}, err
	}

	// LAYER 1: build (cwd inside the module subdir resolves kong from go.mod).
	bin := filepath.Join(dir, "bin")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		return Verdict{Builds: false, Detail: "build: " + tail(string(out))}, nil
	}
	v := Verdict{Builds: true}

	// LAYER 2: registration — the verb resolves and prints help.
	if code, _ := run(bin, verb, "--help"); code == 0 {
		v.Registers = true
	} else {
		v.Detail = "verb not registered"
		return v, nil
	}

	// LAYER 3: contract (behavioral) — only if one applies to this class.
	switch {
	case c != nil && c.EnumFlag != "":
		v.HasContract = true
		// An enforced enum rejects an off-list value (parse-time, exit != 0).
		code, _ := run(bin, verb, "--"+c.EnumFlag, c.OffListVal)
		v.Contract = code != 0
		if !v.Contract {
			v.Detail = "enum --" + c.EnumFlag + " accepted off-list value (constraint dropped)"
		}
	case c != nil && c.NeedFlag != "":
		v.HasContract = true
		// A required flag must exist and be accepted.
		code, _ := run(bin, verb, "--"+c.NeedFlag, "x")
		v.Contract = code == 0
		if !v.Contract {
			v.Detail = "flag --" + c.NeedFlag + " missing (field dropped)"
		}
	}
	return v, nil
}

// run executes the built binary with args and returns its exit code.
func run(bin string, args ...string) (int, string) {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), string(out)
	}
	return -1, string(out)
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return "…" + s[len(s)-200:]
	}
	return s
}
