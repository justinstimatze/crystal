package cmd

import "testing"

// TestCommandSignatureCanonicalizes pins the cluster-key logic: command +
// subcommand + significant flags, paths dropped, and the git-add "stage all"
// synonyms (-A / --all / bare .) folded to one signature — so the flagship rule
// clusters across projects no matter which synonym each memory used.
func TestCommandSignatureCanonicalizes(t *testing.T) {
	tests := []struct {
		span string
		want string
	}{
		{"git add -A", "git add <all>"},
		{"git add --all", "git add <all>"},
		{"git add .", "git add <all>"},
		{"git add path/to/file", "git add"}, // explicit path → not stage-all
		// Flags/paths are dropped: the signature is a CLUSTERING key (group rules by
		// command+subcommand so the same rule clusters across projects regardless of
		// the specific remedy flag); the example line carries the --private/-t detail.
		// The one exception is git-add's stage-all operation, folded to <all> above,
		// because there the flag defines the dangerous OPERATION, not a remedy.
		{"gh repo create --private", "gh repo create"},
		{"git config --global init.defaultBranch", "git config"},
		{"docker build -t app .", "docker build"},
		{"FOO_BAR=1", ""},         // not a command lead → excluded
		{"some/path/file.go", ""}, // a path reference, not a command
		{"the loop", ""},          // prose in backticks
	}
	for _, tc := range tests {
		if got := commandSignature(tc.span); got != tc.want {
			t.Errorf("commandSignature(%q) = %q, want %q", tc.span, got, tc.want)
		}
	}
}

// TestIsRuleLineSeparatesConstraintsFromMentions is the noise filter that makes
// sweep find RULES not command mentions: "never git add -A" is a rule; "run
// python discover.py" is a how-to mention.
func TestIsRuleLineSeparatesConstraintsFromMentions(t *testing.T) {
	rules := []string{
		"never `git add -A`; stage explicit paths",
		"Always create repos as `gh repo create --private`",
		"default to `main` not `master`",
		"do not `git push` to a shared branch",
	}
	mentions := []string{
		"run `python discover.py` to build the schema",
		"the binary is at `go install ./cmd/foo`",
		"5. Push tag: `git push origin v0.x.0`",
	}
	for _, r := range rules {
		if !isRuleLine(r) {
			t.Errorf("isRuleLine(%q) = false, want true (it's a constraint)", r)
		}
	}
	for _, m := range mentions {
		if isRuleLine(m) {
			t.Errorf("isRuleLine(%q) = true, want false (it's a mention)", m)
		}
	}
}
