// Package sediment is the rewrite-gate behind the defn-backed Authorer: it
// locates a function's source span, splices a candidate implementation in place,
// runs the covering tests SCOPED to that package, and ALWAYS restores the
// original file. This is "the frontier authors a replacement, the tests certify
// it" on arbitrary Go logic — the self-authoring gate generalized off kong
// scaffolds. The working tree is never left modified (defer-restore even on
// failure); a passing candidate is PROPOSED, never auto-applied.
package sediment

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// modulePrefix is the repo's module path; defn reports import paths under it.
const modulePrefix = "github.com/justinstimatze/crystal/"

// FuncInfo locates a plain (non-method) function's source span in its file.
type FuncInfo struct {
	File      string // path to the .go file
	Name      string
	DeclSrc   string // full source of the func declaration
	Sig       string // "func Name(params) ret " up to the body brace
	Param0    string // first param name ("" if none)
	RetParam0 bool   // single result whose type == param0's type (identity is a compiling wrong impl)
	start     int    // byte offset of decl start in File
	end       int    // byte offset of decl end in File
}

// repoRel maps a defn module import path to a repo-relative directory.
func repoRel(module string) string {
	if strings.HasPrefix(module, modulePrefix) {
		return strings.TrimPrefix(module, modulePrefix)
	}
	return "." // root module
}

// Locate finds a plain function by name in the module's package directory.
func Locate(module, name string) (FuncInfo, error) {
	dir := repoRel(module)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return FuncInfo{}, err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			return FuncInfo{}, err
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			continue
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Name.Name != name || fd.Body == nil {
				continue // plain functions only (methods are a later step)
			}
			start := fset.Position(fd.Pos()).Offset
			end := fset.Position(fd.End()).Offset
			bodyStart := fset.Position(fd.Body.Pos()).Offset
			info := FuncInfo{
				File: path, Name: name,
				DeclSrc: string(src[start:end]),
				Sig:     string(src[start:bodyStart]),
				start:   start, end: end,
			}
			if fd.Type.Params != nil && len(fd.Type.Params.List) > 0 && len(fd.Type.Params.List[0].Names) > 0 {
				info.Param0 = fd.Type.Params.List[0].Names[0].Name
				if fd.Type.Results != nil && len(fd.Type.Results.List) == 1 {
					p0 := identStr(fd.Type.Params.List[0].Type)
					rt := identStr(fd.Type.Results.List[0].Type)
					info.RetParam0 = p0 != "" && p0 == rt
				}
			}
			return info, nil
		}
	}
	return FuncInfo{}, fmt.Errorf("plain function %q not found in %s", name, dir)
}

func identStr(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// LazyWrong returns a COMPILING but behaviorally-wrong implementation for the
// negative control: identity (return the first param) when the single result
// type matches it, else a panic. The gate must REJECT this — that is the proof
// the covering tests are load-bearing.
func (info FuncInfo) LazyWrong() string {
	if info.RetParam0 && info.Param0 != "" {
		return info.Sig + "{ return " + info.Param0 + " }"
	}
	return info.Sig + `{ panic("crystal-neg-control") }`
}

// ValidateDecl checks that src parses as a single function declaration named name.
func ValidateDecl(name, src string) error {
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package p\n"+src, 0)
	if err != nil {
		return fmt.Errorf("candidate does not parse: %w", err)
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return nil
		}
	}
	return fmt.Errorf("candidate is not a function named %q", name)
}

// GateRewrite splices candidate in place of the function, runs the covering
// tests scoped to pkg, and ALWAYS restores the original file. Returns
// (passed, detail). A non-compiling or failing candidate is passed=false.
func GateRewrite(info FuncInfo, candidate, pkg string, tests []string) (bool, string, error) {
	orig, err := os.ReadFile(info.File)
	if err != nil {
		return false, "", err
	}
	spliced := string(orig[:info.start]) + candidate + string(orig[info.end:])
	if err := os.WriteFile(info.File, []byte(spliced), 0o644); err != nil {
		return false, "", err
	}
	defer os.WriteFile(info.File, orig, 0o644) // ALWAYS restore — never leave the tree modified

	runExpr := "^(" + strings.Join(tests, "|") + ")$"
	if len(tests) == 0 {
		runExpr = "." // no scoped tests → run the package's tests
	}
	out, terr := exec.Command("go", "test", "-run", runExpr, pkg).CombinedOutput()
	if terr != nil {
		return false, tail(string(out)), nil
	}
	return true, "", nil
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		return "…" + s[len(s)-240:]
	}
	return s
}
