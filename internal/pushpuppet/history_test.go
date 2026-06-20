package pushpuppet

import (
	"context"
	"strings"
	"testing"

	"github.com/original-flipster69/koko/internal/provider"
)

func toolMsg(name, path string) provider.Msg {
	args := map[string]string{}
	if path != "" {
		if name == "rename_file" {
			args["new_path"] = path
		} else {
			args["path"] = path
		}
	}
	return provider.Msg{
		Role:      provider.Assistant,
		ToolCalls: []provider.ToolCall{{ID: "id0000001", Name: name, Args: args}},
	}
}

func TestExtractToolOutputFacts_MultilineFormat(t *testing.T) {
	content := "<tool_output name=\"write_file\">\nwrote foo.go\n</tool_output>\n" +
		"<tool_output name=\"replace_in_file\">\nupdated src/main.go\n</tool_output>\n" +
		"<tool_output name=\"read_file\">\n[bar.go lines 1-5 of 5]\nline 1\nline 2\n</tool_output>\n" +
		"<tool_output name=\"exec_command\">\nhello\n</tool_output>\n"

	var modified, read, commands, errors []string
	extractToolOutputFacts(content, &modified, &read, &commands, &errors)

	if got, want := strings.Join(modified, ","), "foo.go,src/main.go"; got != want {
		t.Errorf("modified: got %q, want %q", got, want)
	}
	if got, want := strings.Join(read, ","), "bar.go"; got != want {
		t.Errorf("read: got %q, want %q", got, want)
	}
	if got, want := strings.Join(commands, ","), "(command)"; got != want {
		t.Errorf("commands: got %q, want %q", got, want)
	}
}

func TestExtractToolOutputFacts_ErrorLineCapture(t *testing.T) {
	content := "<tool_output name=\"write_file\">\nerror: refusing to write — privacy detected\n</tool_output>\n"

	var modified, read, commands, errors []string
	extractToolOutputFacts(content, &modified, &read, &commands, &errors)

	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if !strings.Contains(errors[0], "refusing to write") {
		t.Errorf("error content unexpected: %q", errors[0])
	}
}

func TestExtractPathFromContentLine(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"wrote foo.go", "foo.go"},
		{"updated src/main.go", "src/main.go"},
		{"deleted obsolete.txt", "obsolete.txt"},
		{"renamed old.go", "old.go"},
		{"[main.go lines 1-50 of 50]", "main.go"},
		{"hello world", ""},
		{"prefix mismatch nope", ""},
	}
	for _, c := range cases {
		if got := extractPathFromContentLine(c.in); got != c.want {
			t.Errorf("input %q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSummarizeMessages_BasicFacts(t *testing.T) {
	msgs := []provider.Msg{
		{Role: provider.User, Content: "fix the bug in main.go"},
		{Role: provider.Assistant, Content: "looking"},
		toolMsg("read_file", "main.go"),
		toolMsg("write_file", "main.go"),
		{Role: provider.User, Content: "now refactor parser.go"},
		toolMsg("replace_in_file", "parser.go"),
	}
	summary := summarizeMessages(msgs)

	if !strings.Contains(summary, "fix the bug in main.go") {
		t.Errorf("user request missing in summary:\n%s", summary)
	}
	if !strings.Contains(summary, "now refactor parser.go") {
		t.Errorf("second user request missing:\n%s", summary)
	}
	if !strings.Contains(summary, "Files modified: main.go, parser.go") {
		t.Errorf("modified files line wrong:\n%s", summary)
	}
	if !strings.Contains(summary, "Files read: main.go") {
		t.Errorf("read files line wrong:\n%s", summary)
	}
}

func TestSummarizeMessages_LongUserRequestTruncation(t *testing.T) {
	longReq := strings.Repeat("a", maxSummarizedRequestLen+50)
	msgs := []provider.Msg{
		{Role: provider.User, Content: longReq},
	}
	summary := summarizeMessages(msgs)

	if !strings.Contains(summary, strings.Repeat("a", maxSummarizedRequestLen)+"...") {
		t.Errorf("long request not truncated as expected:\n%s", summary)
	}
}

func TestSummarizeMessages_PreservesPreviousSummary(t *testing.T) {
	priorSummary := summarizeMessages([]provider.Msg{
		{Role: provider.User, Content: "earlier request"},
		toolMsg("write_file", "alpha.go"),
		toolMsg("read_file", "beta.go"),
	})

	msgs := []provider.Msg{
		{Role: provider.User, Content: priorSummary},
		{Role: provider.Assistant, Content: trimAck},
		{Role: provider.User, Content: "new request"},
		toolMsg("write_file", "gamma.go"),
	}
	merged := summarizeMessages(msgs)

	if !strings.Contains(merged, "earlier request") {
		t.Errorf("previous user request lost:\n%s", merged)
	}
	if !strings.Contains(merged, "new request") {
		t.Errorf("new user request missing:\n%s", merged)
	}
	if !strings.Contains(merged, "alpha.go") {
		t.Errorf("previous modified file lost:\n%s", merged)
	}
	if !strings.Contains(merged, "gamma.go") {
		t.Errorf("new modified file missing:\n%s", merged)
	}
	if !strings.Contains(merged, "beta.go") {
		t.Errorf("previous read file lost:\n%s", merged)
	}
}

func TestDedupePaths_NormalizesEquivalents(t *testing.T) {
	in := []string{"foo.go", "./foo.go", "src/../foo.go", "bar.go"}
	out := dedupePaths(in)
	if len(out) != 2 {
		t.Errorf("expected 2 unique paths, got %d: %v", len(out), out)
	}
}

func TestTruncateForHistory(t *testing.T) {
	small := "hello"
	if got := truncateForHistory(small); got != small {
		t.Errorf("small input modified: got %q", got)
	}
	big := strings.Repeat("x", maxToolResultSize+100)
	got := truncateForHistory(big)
	if !strings.HasSuffix(got, "\n...(truncated)") {
		t.Errorf("expected truncation suffix on big input")
	}
	if len(got) != maxToolResultSize+len("\n...(truncated)") {
		t.Errorf("unexpected truncated length %d", len(got))
	}
}

func TestEstimateMessagesTokens_Heuristic(t *testing.T) {
	msgs := []provider.Msg{
		{Content: "hi"},
		{Content: "hello there"},
	}
	got := estimateMessagesTokens(msgs)
	if got <= 0 {
		t.Errorf("expected positive estimate, got %d", got)
	}
}

func TestEstimateTokens_UsesCacheWhenSet(t *testing.T) {
	a := &PushPuppet{
		history:         []provider.Msg{{Content: "very long content that the heuristic would otherwise score above the cache value"}},
		lastInputTokens: 42,
	}
	if got := a.estimateTokens(); got != 42 {
		t.Errorf("expected cached value 42, got %d", got)
	}
}

func TestTrimHistory_NoopUnderThreshold(t *testing.T) {
	a := &PushPuppet{history: []provider.Msg{
		{Role: provider.System, Content: "sys"},
		{Role: provider.User, Content: "hi"},
		{Role: provider.Assistant, Content: "hello"},
	}}
	before := len(a.history)
	a.trimHistory(context.Background())
	if len(a.history) != before {
		t.Errorf("history mutated under threshold: %d → %d", before, len(a.history))
	}
}

func TestTrimHistory_PreservesSystemAndEndsAtUser(t *testing.T) {
	bigChunk := strings.Repeat("x", 500_000)
	a := &PushPuppet{history: []provider.Msg{
		{Role: provider.System, Content: "sys"},
		{Role: provider.User, Content: "first"},
		{Role: provider.Assistant, Content: bigChunk},
		{Role: provider.User, Content: "second"},
		{Role: provider.Assistant, Content: "ok"},
		{Role: provider.User, Content: "third"},
		{Role: provider.Assistant, Content: "done"},
	}}
	a.trimHistory(context.Background())

	if a.history[0].Role != provider.System || a.history[0].Content != "sys" {
		t.Errorf("system prompt not preserved at index 0: %+v", a.history[0])
	}
	if a.history[1].Role != provider.User {
		t.Errorf("expected user message at index 1 (summary), got %v", a.history[1].Role)
	}
	if !strings.HasPrefix(a.history[1].Content, summaryHeader) {
		t.Errorf("index 1 not a summary, got: %q", a.history[1].Content)
	}
	if a.history[2].Role != provider.Assistant {
		t.Errorf("expected assistant ack at index 2, got %v", a.history[2].Role)
	}
	if a.history[3].Role != provider.User {
		t.Errorf("expected first kept message to be user-role, got %v", a.history[3].Role)
	}
}

func TestTrimHistory_ScalesToRealTokenCount(t *testing.T) {
	build := func(lastReal int) *PushPuppet {
		h := []provider.Msg{{Role: provider.System, Content: "sys"}}
		for i := 0; i < 12; i++ {
			h = append(h, provider.Msg{Role: provider.User, Content: "req"})
			h = append(h, provider.Msg{Role: provider.Assistant, Content: strings.Repeat("x", 40000)})
		}
		return &PushPuppet{history: h, lastInputTokens: lastReal}
	}

	est := estimateMessagesTokens(build(0).history)

	a := build(est)
	a.trimHistory(context.Background())

	b := build(est * 4)
	b.trimHistory(context.Background())

	if len(b.history) >= len(a.history) {
		t.Errorf("higher real token count must trim more aggressively: scale1 kept %d msgs, scale4 kept %d msgs", len(a.history), len(b.history))
	}
}

func TestLoadSession_BackwardCompatStripsLeadingSystem(t *testing.T) {
	a := &PushPuppet{history: []provider.Msg{
		{Role: provider.System, Content: "CURRENT system prompt"},
	}}

	loaded := []provider.Msg{
		{Role: provider.System, Content: "STALE old system prompt"},
		{Role: provider.User, Content: "hello"},
		{Role: provider.Assistant, Content: "hi"},
	}

	for len(loaded) > 0 && loaded[0].Role == provider.System {
		loaded = loaded[1:]
	}
	newHist := make([]provider.Msg, 0, 1+len(loaded))
	newHist = append(newHist, a.history[0])
	newHist = append(newHist, loaded...)
	a.history = newHist

	if a.history[0].Content != "CURRENT system prompt" {
		t.Errorf("system prompt not preserved after load: %q", a.history[0].Content)
	}
	if a.history[1].Content != "hello" {
		t.Errorf("expected user message after system, got %+v", a.history[1])
	}
}
