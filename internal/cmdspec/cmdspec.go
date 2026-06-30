// Package cmdspec parses a Go package's kong subcommand structs into
// domain-neutral specs: (name, flags, shape-features). It is the corpus-parser
// seam of the dogfood harness — the keepable tooling. Swap this parser and the
// same loop crystallizes a different codegen chore; nothing here is specific to
// crystal beyond "kong command struct" conventions.
package cmdspec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// FlagSpec is one struct field of a command (one CLI flag).
type FlagSpec struct {
	Name    string // Go field name (e.g. "CacheDir")
	Type    string // one of: string, int, int64, bool, float64, []string
	Help    string
	Default string
	Enum    string // comma-separated allowed values, "" if unconstrained
}

// Repeatable reports whether the flag is a slice (kong accepts it repeatedly).
func (f FlagSpec) Repeatable() bool { return f.Type == "[]string" }

// CmdSpec is one parsed subcommand: its struct name and flags, plus the shape
// features that drive the regular/irregular split.
type CmdSpec struct {
	Name   string // struct name, e.g. "ProbeCmd"
	Fields []FlagSpec
}

// HasEnum reports whether any flag is enum-constrained.
func (c CmdSpec) HasEnum() bool {
	for _, f := range c.Fields {
		if f.Enum != "" {
			return true
		}
	}
	return false
}

// HasSlice reports whether any flag is a repeatable slice.
func (c CmdSpec) HasSlice() bool {
	for _, f := range c.Fields {
		if f.Repeatable() {
			return true
		}
	}
	return false
}

// Irregular flags a command whose shape a generator trained on plain
// scalar-flag commands was never shown.
func (c CmdSpec) Irregular() bool { return c.HasEnum() || c.HasSlice() }

// Verb returns the kong CLI verb derived from the struct name (camel→kebab of
// the name minus the trailing "Cmd").
func (c CmdSpec) Verb() string { return Kebab(strings.TrimSuffix(c.Name, "Cmd")) }

// Parse reads every *.go file in dir and returns a spec for each
// `type XCmd struct { ... }` (excluding the root CLI struct). Fields with an
// unsupported type are skipped (recorded by absence, never silently coerced).
func Parse(dir string) ([]CmdSpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var specs []CmdSpec
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				// Exported `XCmd` structs only — skips unexported helpers like
				// `labeledCmd` that happen to share the suffix.
				if !ok || !strings.HasSuffix(ts.Name.Name, "Cmd") || !ast.IsExported(ts.Name.Name) {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				specs = append(specs, CmdSpec{Name: ts.Name.Name, Fields: fields(st)})
			}
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs, nil
}

func fields(st *ast.StructType) []FlagSpec {
	var out []FlagSpec
	for _, fld := range st.Fields.List {
		if len(fld.Names) != 1 { // skip embedded/anonymous
			continue
		}
		typ, ok := typeString(fld.Type)
		if !ok {
			continue
		}
		fs := FlagSpec{Name: fld.Names[0].Name, Type: typ}
		if fld.Tag != nil {
			raw := strings.Trim(fld.Tag.Value, "`")
			tag := reflect.StructTag(raw)
			fs.Help = tag.Get("help")
			fs.Default = tag.Get("default")
			fs.Enum = tag.Get("enum")
		}
		out = append(out, fs)
	}
	return out
}

func typeString(e ast.Expr) (string, bool) {
	switch t := e.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string", "int", "int64", "bool", "float64":
			return t.Name, true
		}
	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && id.Name == "string" {
			return "[]string", true
		}
	}
	return "", false
}

// Kebab lowercases an exported Go identifier into kong's flag/verb form
// (camelCase → kebab-case). Approximate for acronyms; exact for the
// single-capital-word fields the contracts target.
func Kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
