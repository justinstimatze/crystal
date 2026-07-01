// Package chores holds SYNTHETIC kong command structs used only as planshift
// chores. They are never compiled into crystal (the Go toolchain ignores any
// directory named testdata), never registered in the CLI, and exist solely to
// give `crystal planshift --chore-dir` a MIDDLE-BAND corpus that crystal's real
// commands can't supply.
//
// The band planshift needs: hard enough that the weak executor drops an
// irregular tag WITHOUT a plan, gettable WITH one. crystal's real enum commands
// are all large (7–9 flags) and floor the weak model no matter the plan; its
// small commands are pure slice-flags and pass with no plan. Neither
// discriminates. These chores are graduated in irregular-tag COUNT (the attrition
// axis) at small-to-moderate flag counts, so the none-arm should start failing
// (an enum silently dropped under output length) while a flag-by-flag plan holds
// each tag in place. If plans still don't help here, that is an honest negative:
// a complete spec leaves a plan nothing to add.
package chores

// RouteCmd — 3 flags, 1 enum. The floor probe: a single enum next to plain
// flags. If the weak model can't hold ONE enum tag, nothing below will pass.
type RouteCmd struct {
	Path    string `arg:"" help:"URL path to route."`
	Method  string `help:"HTTP method." enum:"get,post,put,delete,patch"`
	Verbose bool   `help:"Log each matched route."`
}

// ConvertCmd — 3 flags, 2 enums. Two enums that share a value vocabulary
// (from/to over the same set) are the classic conflation trap — the weak model
// tends to tag one and forget the other.
type ConvertCmd struct {
	Input string `arg:"" help:"File to convert."`
	From  string `help:"Source format." enum:"json,yaml,toml,csv"`
	To    string `help:"Target format." enum:"json,yaml,toml,csv"`
}

// NotifyCmd — 4 flags, 2 enums + 1 slice. Mixed irregular shapes at once: the
// model must keep two enum tags AND a repeatable slice, the combination the
// dogfood harness found generators drop first.
type NotifyCmd struct {
	Message  string   `arg:"" help:"Message body."`
	Channel  string   `help:"Delivery channel." enum:"email,sms,push,slack,webhook"`
	Severity string   `help:"Severity level." enum:"debug,info,warn,error"`
	Tags     []string `help:"Repeatable routing tags."`
}

// DeployCmd — 5 flags, 3 enums + 1 slice. Moderate attrition load: three enums
// is where a bare weak-model pass typically drops at least one tag.
type DeployCmd struct {
	Target   string   `help:"Deploy target." enum:"dev,staging,prod"`
	Strategy string   `help:"Rollout strategy." enum:"rolling,bluegreen,canary"`
	Region   string   `help:"Region." enum:"us-east,us-west,eu,asia"`
	Services []string `help:"Repeatable service names."`
	DryRun   bool     `help:"Plan only; do not apply."`
}
