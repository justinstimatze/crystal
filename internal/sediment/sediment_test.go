package sediment

import "testing"

func TestValidateDecl(t *testing.T) {
	ok := "func Kebab(s string) string { return s }"
	if err := ValidateDecl("Kebab", ok); err != nil {
		t.Errorf("valid func rejected: %v", err)
	}
	if err := ValidateDecl("Kebab", "not go {{{"); err == nil {
		t.Error("unparseable candidate accepted")
	}
	if err := ValidateDecl("Kebab", "func Other() {}"); err == nil {
		t.Error("wrong-name candidate accepted")
	}
}

func TestLazyWrong(t *testing.T) {
	// string->string: identity is a compiling wrong impl.
	id := FuncInfo{Name: "Kebab", Sig: "func Kebab(s string) string ", Param0: "s", RetParam0: true}
	if got := id.LazyWrong(); got != "func Kebab(s string) string { return s }" {
		t.Errorf("identity LazyWrong = %q", got)
	}
	// non-matching return type: fall back to panic.
	pn := FuncInfo{Name: "Count", Sig: "func Count(s string) int ", Param0: "s", RetParam0: false}
	if got := pn.LazyWrong(); got != `func Count(s string) int { panic("crystal-neg-control") }` {
		t.Errorf("panic LazyWrong = %q", got)
	}
}

func TestLocate(t *testing.T) {
	// Locate reads a single file directly (defn's source_file).
	info, err := Locate("../cmdspec/cmdspec.go", "Kebab")
	if err != nil {
		t.Fatalf("locate Kebab: %v", err)
	}
	if info.Param0 != "s" || !info.RetParam0 {
		t.Errorf("Kebab is string->string: Param0=%q RetParam0=%v", info.Param0, info.RetParam0)
	}
	if info.LazyWrong() != "func Kebab(s string) string { return s }" {
		t.Errorf("LazyWrong = %q", info.LazyWrong())
	}
}
