package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func runMistralStream(t *testing.T, sse string) *Response {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	t.Cleanup(srv.Close)

	m, err := newMistral("key", "mistral-medium-3-5", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := m.ChatStream(context.Background(), []Msg{{Role: User, Content: "hi"}}, nil, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	return resp
}

func TestMistralStreamEmptyArgsToolNotDropped(t *testing.T) {
	sse := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"list_memories","arguments":""}}]}}]}
data: [DONE]
`
	resp := runMistralStream(t, sse)
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "list_memories" {
		t.Errorf("name = %q, want list_memories", resp.ToolCalls[0].Name)
	}
	if len(resp.ToolCalls[0].Args) != 0 {
		t.Errorf("expected empty args, got %v", resp.ToolCalls[0].Args)
	}
}

func TestMistralStreamCollidingIndices(t *testing.T) {
	sse := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"read_file","arguments":"{\"path\":\"a\"}"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"read_file","arguments":"{\"path\":\"b\"}"}}]}}]}
data: [DONE]
`
	resp := runMistralStream(t, sse)
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Args["path"] != "a" || resp.ToolCalls[1].Args["path"] != "b" {
		t.Errorf("paths = %q, %q; want a, b", resp.ToolCalls[0].Args["path"], resp.ToolCalls[1].Args["path"])
	}
}

func TestMistralStreamNonContiguousIndices(t *testing.T) {
	sse := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"search_files","arguments":"{\"pattern\":\"x\"}"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":2,"function":{"name":"list_dir","arguments":"{\"path\":\".\"}"}}]}}]}
data: [DONE]
`
	resp := runMistralStream(t, sse)
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
	names := resp.ToolCalls[0].Name + "," + resp.ToolCalls[1].Name
	if names != "search_files,list_dir" {
		t.Errorf("names = %q, want search_files,list_dir", names)
	}
}

func TestMistralStreamFragmentedArgs(t *testing.T) {
	sse := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"write_file","arguments":"{\"path\":"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"","arguments":"\"f\",\"content\":\"hi\"}"}}]}}]}
data: [DONE]
`
	resp := runMistralStream(t, sse)
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	args := resp.ToolCalls[0].Args
	if args["path"] != "f" || args["content"] != "hi" {
		t.Errorf("args = %v, want path=f content=hi", args)
	}
}
