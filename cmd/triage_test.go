package cmd
import "testing"
func TestLeadingWrapperKeywordUnmasksAction(t *testing.T) {
	cases := map[string]string{
		`until curl -s -o /dev/null -w "%{http_code}" http://localhost:5173 | grep -q 200; do sleep 0.3; done`: "network",
		`time go test ./...`: "build/test",
		`while curl -sf http://localhost:3000; do sleep 1; done`: "network",
		`command -v nc`: "", // MUST stay unmasked-safe: 'command' is NOT stripped (the -v idiom)
	}
	for cmd, want := range cases {
		if got := detClassify(cmd); got != want {
			t.Errorf("detClassify(%.40q) = %q, want %q", cmd, got, want)
		}
	}
}
