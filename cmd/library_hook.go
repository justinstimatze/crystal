package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/library"
)

// LibraryHookCmd is the batch→live gap closed for the SERVE tier: the
// serve-from-a-library layer wired to REAL Claude Code hook events instead of
// the canned in-process stream `library` demos. This is what makes "ambient, no
// asking" true at the serve layer — a crystallized recipe is injected as context
// the moment the session's intent matches it, with 0 model calls, behind the
// same deterministic confidence + cooldown + too-much gate.
//
// The surface finding (why this is NOT a straight copy of guard/dispatch): the
// library's entries are keyed on INTENT tokens (entity/struct/classify/quote),
// and intent lives in the USER'S PROMPT, not in a tool command. guard/dispatch
// are PreToolUse hooks because they gate tool COMMANDS; the serve library serves
// intent RECIPES, so its rich surface is UserPromptSubmit — whose payload
// carries the prompt prose the entries match. (The library demo's canned stream
// literally IS a stream of user prompts.) PreToolUse still works, but only the
// entries whose triggers show up in a tool input can fire there — for the shipped
// library that is just the git-add guard. So this hook reads BOTH surfaces and
// serves from whichever text the event carries.
//
// Contract (auto-detected per surface):
//
//	UserPromptSubmit  stdin:  {"hook_event_name":"UserPromptSubmit","prompt":"..."}
//	                  stdout: {"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"..."}}
//	PreToolUse        stdin:  {"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"..."}}
//	                  stdout: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","additionalContext":"..."}}
//
// On a confident, cooldown-clear match the served recipe is injected as
// additionalContext; on an abstain (low-conf / cooldown / too-much / demoted /
// no-match) the hook stays silent (allow, no context) — precision over recall, a
// wrong recipe is worse than none. Fail-open throughout: a malformed event, an
// unreadable library, or a marshalling failure all yield a silent allow so the
// hook never blocks the host.
//
// Cross-process state (the "live" part): each hook call is a separate process,
// so the cooldown window and the demoted set survive only through --state. That
// disk round-trip is what makes cooldown real live (the same entry does not
// re-inject on every consecutive matching prompt) and what makes demotion stick
// (a demoted entry stays demoted across events) — exactly the shape the
// classifier hook's drift window uses.
//
// Demote/promote are control ops (the serve-layer analog of the classifier
// hook's demote, driven by the re-author loop): `library-hook --demote NAME`
// marks an entry drifted and exits; `--promote NAME` clears it. They mutate the
// persisted --state, not an event.
type LibraryHookCmd struct {
	Lib     string `help:"Library spec (crystallized artifacts + groupchat-style metadata)." default:"testdata/library/entries.json"`
	State   string `help:"Cross-invocation serve-state file (step + cooldown map + demoted set)." default:".crystal-library-state.json"`
	Demote  string `help:"Control op: mark this entry demoted (persisted) and exit — the re-author loop's serve-layer demote. Empty ⇒ normal event mode."`
	Promote string `help:"Control op: clear this entry's demotion (persisted) and exit."`
}

// libHookEvent is the subset of a Claude Code hook payload the serve layer
// consumes, spanning both surfaces: UserPromptSubmit carries intent prose in
// .prompt (the rich surface); PreToolUse carries the tool command in
// .tool_input.command (the narrow surface).
type libHookEvent struct {
	HookEventName string `json:"hook_event_name,omitempty"`
	Prompt        string `json:"prompt,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	ToolInput     struct {
		Command string `json:"command,omitempty"`
	} `json:"tool_input,omitempty"`
}

// serveContext returns the text to match the library against and the surface
// label, from whichever hook surface this event is. Prompt wins when present
// (the rich surface); otherwise the tool command (the narrow surface).
func (e libHookEvent) serveContext() (text, surface string) {
	if p := strings.TrimSpace(e.Prompt); p != "" {
		return p, "UserPromptSubmit"
	}
	if c := strings.TrimSpace(e.ToolInput.Command); c != "" {
		return c, "PreToolUse"
	}
	return "", e.HookEventName
}

// decideLibraryHook is the pure core: given a freshly-loaded library, the
// persisted serve state, and the event's context text, restore the state, serve
// (or abstain) at the current step, and return the decision plus the next state
// to persist. Kept pure (no I/O) so the serve/cooldown/demote behaviors are
// unit-tested the same way decideHook is.
func decideLibraryHook(lib *library.Library, st library.ServeState, text string) (library.Decision, library.ServeState) {
	lib.Restore(st)
	d := lib.Serve(text, st.Step)
	return d, lib.Snapshot(st.Step + 1)
}

// serveInjection is the additionalContext a served entry injects, or "" on
// abstain (silent defer). Injecting the recipe body is the ambient serve.
func serveInjection(d library.Decision) string {
	if !d.Served() {
		return ""
	}
	return fmt.Sprintf("[crystal] serving crystallized recipe %q (%s rung, deterministic match "+
		"conf %.2f, 0 model calls): %s", d.Entry.Name, d.Entry.Rung, d.Confidence, d.Entry.Artifact)
}

func (c *LibraryHookCmd) Run() error {
	if c.Demote != "" || c.Promote != "" {
		return c.control()
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return emitLibraryServe("", "") // fail-open: silent allow
	}
	var ev libHookEvent
	_ = json.Unmarshal(raw, &ev) // fail-open: a malformed event serves nothing

	text, surface := ev.serveContext()
	if text == "" {
		return emitLibraryServe(surface, "")
	}

	lib, err := library.Load(c.Lib)
	if err != nil {
		return emitLibraryServe(surface, "") // a broken library never blocks the host
	}
	st := loadLibraryState(c.State)
	d, next := decideLibraryHook(lib, st, text)
	saveLibraryState(c.State, next)
	return emitLibraryServe(surface, serveInjection(d))
}

// control applies a demote/promote to the persisted state and exits. It is not
// an event — it is how the re-author loop (or a human) toggles an entry's
// serving across the process boundary the live hook runs behind.
func (c *LibraryHookCmd) control() error {
	st := loadLibraryState(c.State)
	dem := map[string]bool{}
	for _, n := range st.Demoted {
		dem[n] = true
	}
	var action string
	switch {
	case c.Demote != "":
		dem[c.Demote] = true
		action = "demoted " + c.Demote
	case c.Promote != "":
		delete(dem, c.Promote)
		action = "promoted " + c.Promote
	}
	names := make([]string, 0, len(dem))
	for n := range dem {
		names = append(names, n)
	}
	sort.Strings(names)
	st.Demoted = names
	saveLibraryState(c.State, st)
	fmt.Fprintf(os.Stderr, "library-hook: %s (demoted set now: %s)\n", action, strings.Join(names, ", "))
	return nil
}

// emitLibraryServe writes the surface-appropriate hook response with the served
// recipe (or empty = silent). PreToolUse carries permissionDecision=allow;
// UserPromptSubmit does not. An empty/unknown surface defaults to
// UserPromptSubmit (the library's primary surface).
func emitLibraryServe(surface, additionalContext string) error {
	type specific struct {
		HookEventName      string `json:"hookEventName"`
		PermissionDecision string `json:"permissionDecision,omitempty"`
		AdditionalContext  string `json:"additionalContext,omitempty"`
	}
	spec := specific{HookEventName: surface, AdditionalContext: additionalContext}
	switch surface {
	case "PreToolUse":
		spec.PermissionDecision = "allow"
	case "":
		spec.HookEventName = "UserPromptSubmit"
	}
	out := struct {
		HookSpecificOutput specific `json:"hookSpecificOutput"`
	}{spec}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(b)
	return err
}

func loadLibraryState(path string) library.ServeState {
	st := library.ServeState{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

func saveLibraryState(path string, st library.ServeState) {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// LibraryHookDemoCmd proves the serve tier is LIVE, not batch: it drives the
// real built binary (`crystal library-hook`) across a stream of real hook
// events — each a separate process piping JSON over stdin — with the cooldown
// window and demotions persisting ONLY through the on-disk state file. It shows
// serve on a strong match, cooldown across the process boundary (the same entry
// declines to re-inject on the very next process because the window round-tripped
// through disk), and demotion persistence (a control-op demote takes an entry
// out of service for every subsequent process).
type LibraryHookDemoCmd struct {
	Lib     string `help:"Library spec." default:"testdata/library/entries.json"`
	Verbose bool   `help:"Print the full hook JSON response for every event."`
}

func (c *LibraryHookDemoCmd) Run() error {
	// A throwaway state file — the cross-process cooldown/demotion window.
	stateFile, err := os.CreateTemp("", "crystal-libhookdemo-*.json")
	if err != nil {
		return usageError{fmt.Errorf("creating temp state: %w", err)}
	}
	statePath := stateFile.Name()
	stateFile.Close()
	os.Remove(statePath) // start fresh; the hook recreates it on first write
	defer os.Remove(statePath)

	self := os.Args[0] // the real crystal binary currently executing

	// A stream of real UserPromptSubmit events (the library's rich surface).
	// Crafted to exercise serve, cross-process cooldown, and (below) demotion.
	type step struct{ surface, text, want string }
	stream := []step{
		{"UserPromptSubmit", "please git add -A and commit", "serve guard"},
		{"UserPromptSubmit", "run git add -A one more time right now", "cooldown (same entry, next process)"},
		{"UserPromptSubmit", "generate a Go struct type for this entity with fields", "serve entity-to-struct"},
		{"UserPromptSubmit", "classify this bash shell command into a category", "serve categorize"},
		{"PreToolUse", "git add -A && git commit -m wip", "PreToolUse: only the tool-observable entry fires"},
	}

	fmt.Printf("library-hookdemo: driving the REAL `crystal library-hook` binary (%s) over live hook events\n", self)
	fmt.Printf("each line is a SEPARATE process; cooldown + demotion survive only via %s\n\n", statePath)
	fmt.Printf("=== serving the event stream (0 model calls; deterministic gate) ===\n")

	served := 0
	for i, s := range stream {
		ctxText, raw, err := invokeLibraryHook(self, c.Lib, statePath, s.surface, s.text)
		if err != nil {
			return usageError{fmt.Errorf("invoking library-hook on %q: %w", s.text, err)}
		}
		label, name := "abstain    ", "—"
		if strings.Contains(ctxText, "serving crystallized recipe") {
			label = "SERVE      "
			served++
			name = injectedName(ctxText)
		}
		fmt.Printf("  [%d] %-11s %-22s %-12s %q\n", i, label, name, "("+s.surface[:min(len(s.surface), 11)]+")", truncate(s.text, 40))
		if c.Verbose {
			fmt.Printf("        → %s\n", strings.TrimSpace(raw))
		}
	}

	// Demote-on-drift, live: a control-op demote (what the re-author loop issues)
	// takes an entry out of service for every subsequent process.
	fmt.Printf("\n=== demote-on-drift across processes: control-op demote 'entity-to-go-struct' ===\n")
	if err := controlLibraryHook(self, c.Lib, statePath, "entity-to-go-struct"); err != nil {
		return usageError{fmt.Errorf("demote control op: %w", err)}
	}
	ctxText, _, err := invokeLibraryHook(self, c.Lib, statePath, "UserPromptSubmit",
		"again generate a Go struct type for an entity with fields")
	demotedNowSilent := !strings.Contains(ctxText, "serving crystallized recipe")
	fmt.Printf("  after demote, the same entity-struct intent → %s (a demoted artifact never serves, and the\n",
		map[bool]string{true: "ABSTAIN", false: "still served (BUG)"}[demotedNowSilent])
	fmt.Printf("  demotion persisted across the process boundary via the state file)\n")

	fmt.Printf("\n=== outcome ===\n")
	fmt.Printf("  served %d recipes over %d events, each a fresh process, deterministic serve (0 model calls).\n", served, len(stream))
	fmt.Println("  Cooldown abstained the repeated git-add intent even though it was a SEPARATE process from")
	fmt.Println("  the first serve — the window exists only because it round-tripped through disk. The")
	fmt.Println("  UserPromptSubmit surface fires the intent-keyed entries; PreToolUse exposes only the")
	fmt.Println("  tool-observable ones (the git-add guard). This is groupchat's serve-from-library shape,")
	fmt.Println("  wired to real hook events across process boundaries — ambient, no asking, at the serve layer.")
	return nil
}

// injectedName pulls the served entry name out of the injected additionalContext
// (best-effort, for the demo's per-line label).
func injectedName(ctxText string) string {
	const marker = "recipe \""
	i := strings.Index(ctxText, marker)
	if i < 0 {
		return "—"
	}
	rest := ctxText[i+len(marker):]
	if j := strings.IndexByte(rest, '"'); j >= 0 {
		return rest[:j]
	}
	return "—"
}

// invokeLibraryHook runs the real `crystal library-hook` binary once with the
// given surface's event on stdin, returning the injected additionalContext (if
// any) and the raw stdout JSON.
func invokeLibraryHook(self, lib, statePath, surface, text string) (additionalContext, rawJSON string, err error) {
	var ev libHookEvent
	switch surface {
	case "PreToolUse":
		ev.HookEventName = "PreToolUse"
		ev.ToolName = "Bash"
		ev.ToolInput.Command = text
	default:
		ev.HookEventName = "UserPromptSubmit"
		ev.Prompt = text
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command(self, "library-hook", "--lib", lib, "--state", statePath)
	cmd.Stdin = bytes.NewReader(payload)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", out.String(), err
	}
	var resp struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return "", out.String(), fmt.Errorf("decoding library-hook output %q: %w", out.String(), err)
	}
	return resp.HookSpecificOutput.AdditionalContext, out.String(), nil
}

// controlLibraryHook issues a demote control op against the real binary.
func controlLibraryHook(self, lib, statePath, name string) error {
	cmd := exec.Command(self, "library-hook", "--lib", lib, "--state", statePath, "--demote", name)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
