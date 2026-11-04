package publicai

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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

func TestReadStreamAssemblesContentReasoningUsageAndFinish(t *testing.T) {
	sse := "data: {\"choices\":[{\"delta\":{\"reasoning\":\"think \"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":[{\"type\":\"thinking\",\"thinking\":[{\"type\":\"text\",\"text\":\"more\"}]},{\"type\":\"text\",\"text\":\"lo\"}]},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":9}}\n\n" +
		"data: [DONE]\n\n"
	r, retry, err := readStream(strings.NewReader(sse), "m", time.Now())
	if err != nil || retry || r.Text != "hello" || r.FinishReason != "stop" || r.InputTokens != 7 || r.OutputTokens != 9 {
		t.Fatalf("%+v retry=%v err=%v", r, retry, err)
	}
}

func TestReadStreamWithoutFinishIsRetryable(t *testing.T) {
	_, retry, err := readStream(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"), "m", time.Now())
	if err == nil || !retry {
		t.Fatal("a stream cut before finish_reason must be a retryable error, never a reply")
	}
}
