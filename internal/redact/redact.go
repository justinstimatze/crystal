// Package redact scrubs secrets and user-identifying paths from extracted
// Records before they are written to the committed corpus.
//
// Why this is load-bearing: a sample of the raw corpus already contained a
// live `sk-ant-api03-…` key and Bearer tokens. The crystal value prop is
// sovereignty; committing verbatim transcript bytes to git would bake
// credentials into reflog across every clone — the opposite of that goal.
//
// Design constraints:
//   - Length-preserving: a secret of N chars becomes a placeholder of N
//     chars so per-tool fidelity math (byte/length comparisons) is
//     unchanged between a redacted historical Record and a redacted
//     produced Record.
//   - Fail-loud: Verify reports any field that still matches a secret
//     pattern after scrubbing; the extractor refuses to write if so.
package redact

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
)

// secretPatterns are gitleaks-style detectors for high-confidence secret
// shapes. Order does not matter; all are applied.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`),                                                // Anthropic
	regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),                                                      // OpenAI-style
	regexp.MustCompile(`ghp_[A-Za-z0-9]{30,}`),                                                     // GitHub PAT
	regexp.MustCompile(`gho_[A-Za-z0-9]{30,}`),                                                     // GitHub OAuth
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                                                         // AWS access key id
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),                                             // Slack
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),                                                    // Google API key
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]{16,}`),                                        // Bearer tokens
	regexp.MustCompile(`-----BEGIN[A-Z ]*PRIVATE KEY-----[\s\S]*?-----END[A-Z ]*PRIVATE KEY-----`), // PEM blocks
}

// homePath matches an absolute home directory (Linux /home and macOS /Users,
// the latter for third-party paths that surface in pasted content) so we can
// normalize it to $HOME — both a privacy measure (strips usernames, including
// other people's) and a determinism one (paths become host-independent).
var homePath = regexp.MustCompile(`/(home|Users)/[A-Za-z0-9._-]+`)

// userRe masks bare username tokens that the home-path normalizer cannot
// reach: `ls -la` ownership columns ("alice alice"), the encoded
// project-dir basenames Claude Code uses ("-home-alice-Documents-foo"),
// and any other free-standing occurrence. Configured via SetUsernames;
// defaults to the running user so extraction on this host self-cleans.
var userRe *regexp.Regexp

func init() {
	if u := filepath.Base(os.Getenv("HOME")); u != "" && u != "." && u != string(filepath.Separator) {
		SetUsernames([]string{u})
	}
}

// SetUsernames configures bare-username masking. Pass every username whose
// bare token should be scrubbed (e.g. both the current host's user and any
// user that appears in historical transcripts from another machine). Idempotent.
func SetUsernames(users []string) {
	quoted := make([]string, 0, len(users))
	for _, u := range users {
		if u != "" {
			quoted = append(quoted, regexp.QuoteMeta(u))
		}
	}
	if len(quoted) == 0 {
		userRe = nil
		return
	}
	userRe = regexp.MustCompile(`\b(` + strings.Join(quoted, "|") + `)\b`)
}

// mask replaces the matched secret with same-length 'x' runes, preserving
// any leading scheme token so the shape stays recognizable as redacted.
func mask(s string) string {
	return strings.Repeat("x", len([]rune(s)))
}

// scrubString applies all secret patterns then home normalization to a
// single string.
func scrubString(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllStringFunc(s, mask)
	}
	s = homePath.ReplaceAllString(s, "$$HOME")
	if userRe != nil {
		s = userRe.ReplaceAllStringFunc(s, mask)
	}
	return s
}

// Record scrubs every text-bearing field of r in place: the typed Output
// text fields, the raw JSON bytes, the structured patch, and the
// surrounding Context/Followup. Args values that are strings are scrubbed
// too (commands and file paths live there).
func Record(r *record.Record) {
	r.Repo = scrubString(r.Repo)
	r.GitBranch = scrubString(r.GitBranch)
	r.Context = scrubString(r.Context)
	r.Followup = scrubString(r.Followup)
	for _, p := range r.Result.TextFields() {
		*p = scrubString(*p)
	}
	if len(r.Result.Raw) > 0 {
		r.Result.Raw = []byte(scrubString(string(r.Result.Raw)))
	}
	if len(r.Result.StructuredPatch) > 0 {
		r.Result.StructuredPatch = []byte(scrubString(string(r.Result.StructuredPatch)))
	}
	for k, v := range r.Args {
		if s, ok := v.(string); ok {
			r.Args[k] = scrubString(s)
		}
	}
}

// Verify returns a non-nil error if any text-bearing field of r still
// matches a secret pattern. The extractor calls this after Record and
// aborts the whole write if it fails — fail-loud, never ship a leak.
func Verify(r *record.Record) error {
	fields := map[string]string{
		"context":  r.Context,
		"followup": r.Followup,
		"raw":      string(r.Result.Raw),
		"patch":    string(r.Result.StructuredPatch),
	}
	for _, p := range r.Result.TextFields() {
		fields["output"] += *p + "\n"
	}
	for k, v := range r.Args {
		if s, ok := v.(string); ok {
			fields["arg:"+k] = s
		}
	}
	for name, val := range fields {
		for _, re := range secretPatterns {
			if loc := re.FindString(val); loc != "" {
				return fmt.Errorf("redact: secret survived in %s field (tool_use_id=%s)", name, r.ToolUseID)
			}
		}
	}
	return nil
}

// Warnf writes a fail-loud warning to stderr. Used by the extractor for
// per-file shortfalls and abort messages.
func Warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "crystal: "+format+"\n", args...)
}
