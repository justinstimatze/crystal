package publicai

import (
	"encoding/json"
	"testing"
)

func TestSplitContentReadsStringsAndMistralThinkingBlocks(t *testing.T) {
	if txt, th := splitContent(json.RawMessage(`"plain"`)); txt != "plain" || th != "" {
		t.Fatalf("%q %q", txt, th)
	}
	raw := `[{"type":"thinking","thinking":[{"type":"text","text":"step 1. "},{"type":"text","text":"step 2."}]},{"type":"text","text":"answer"}]`
	if txt, th := splitContent(json.RawMessage(raw)); txt != "answer" || th != "step 1. step 2." {
		t.Fatalf("%q %q", txt, th)
	}
}
