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

// Contract is the optional behavioral probe for a command's shape class. The
// build-probe verifier uses EnumFlag/NeedFlag; the test verifier additionally
// uses SliceField (the Go field name) to assert binding cardinality.
type Contract struct {
	EnumFlag   string // kebab CLI flag that must reject OffListVal (exit != 0)
	OffListVal string
	NeedFlag   string // kebab CLI flag that must be accepted (exit == 0)
	SliceField string // Go field name of the repeatable flag (test verifier: must bind 2 values)
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

// TestContract is the defn-`test` SHAPE implemented with raw `go test`: it
// writes the generated struct plus a kong.New().Parse() test harness that
// asserts the contract as a TEST (enum off-list → parse error; a repeatable flag
// → binds 2 values), and runs `go test`. It is strictly stronger than the
// binary-probe contract — the slice repetition-binding assertion catches a
// []string rendered as a scalar string, which a flag-presence probe cannot see.
// Returns (passed, detail, err). A non-compiling or failing test is `passed=false`,
// never an error (a wrong scaffold is a finding, not a harness failure).
func TestContract(scratchRoot, structName, field, verb, structSrc string, c *Contract) (bool, string, error) {
	dir := filepath.Join(scratchRoot, verb+"-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, "", err
	}
	gen := fmt.Sprintf("package g\n\n%s\n\ntype Root struct {\n\t%s %s `cmd:\"\" name:%q help:\"g\"`\n}\n", structSrc, field, structName, verb)
	if err := os.WriteFile(filepath.Join(dir, "gen.go"), []byte(gen), 0o644); err != nil {
		return false, "", err
	}
	var body strings.Builder
	if c.EnumFlag != "" {
		fmt.Fprintf(&body, "\t{\n\t\tvar r Root\n\t\tk, err := kong.New(&r)\n\t\tif err != nil {\n\t\t\tt.Fatalf(\"kong.New: %%v\", err)\n\t\t}\n\t\tif _, perr := k.Parse([]string{%q, \"--%s\", %q}); perr == nil {\n\t\t\tt.Fatal(\"enum off-list value accepted (constraint dropped)\")\n\t\t}\n\t}\n", verb, c.EnumFlag, c.OffListVal)
	}
	if c.NeedFlag != "" && c.SliceField != "" {
		fmt.Fprintf(&body, "\t{\n\t\tvar r Root\n\t\tk, err := kong.New(&r)\n\t\tif err != nil {\n\t\t\tt.Fatalf(\"kong.New: %%v\", err)\n\t\t}\n\t\tif _, perr := k.Parse([]string{%q, \"--%s\", \"a\", \"--%s\", \"b\"}); perr != nil {\n\t\t\tt.Fatalf(\"repeatable flag rejected: %%v\", perr)\n\t\t}\n\t\tif len(r.%s.%s) != 2 {\n\t\t\tt.Fatalf(\"flag did not bind 2 values (got %%d) — rendered as scalar, not a slice\", len(r.%s.%s))\n\t\t}\n\t}\n", verb, c.NeedFlag, c.NeedFlag, field, c.SliceField, field, c.SliceField)
	}
	test := "package g\n\nimport (\n\t\"testing\"\n\n\t\"github.com/alecthomas/kong\"\n)\n\nfunc TestContract(t *testing.T) {\n" + body.String() + "}\n"
	if err := os.WriteFile(filepath.Join(dir, "gen_test.go"), []byte(test), 0o644); err != nil {
		return false, "", err
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, tail(string(out)), nil
	}
	return true, "", nil
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
