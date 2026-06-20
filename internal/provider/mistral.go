package provider

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type mistral struct {
	apiKey   string
	model    string
	baseURL  string
	cacheKey string
	effort   Effort
	client   *http.Client
}

func newMistral(apiKey, model, baseURL string) (*mistral, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("MISTRAL_API_KEY not set")
	}
	if baseURL == "" {
		baseURL = "https://api.mistral.ai/v1"
	}
	return &mistral{
		apiKey:   apiKey,
		model:    model,
		baseURL:  baseURL,
		cacheKey: newCacheKey(),
		client:   &http.Client{},
	}, nil
}

func newCacheKey() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "koko-session"
	}
	return "koko-" + hex.EncodeToString(buf)
}

func (m *mistral) Name() string       { return "mistral" }
func (m *mistral) Model() string      { return m.model }
func (m *mistral) SetModel(s string)  { m.model = s }
func (m *mistral) Effort() Effort     { return m.effort }
func (m *mistral) SetEffort(e Effort) { m.effort = e }

// mistralReasoningEffort maps the abstract effort level onto Mistral's
// reasoning_effort, which only accepts "high" or "none". low collapses to
// "none"; medium and high both map to "high". Returns "" to omit the field.
// Note: native-reasoning models (magistral) reject this with HTTP 422.
func mistralReasoningEffort(e Effort) string {
	switch e {
	case EffortLow:
		return "none"
	case EffortMedium, EffortHigh:
		return "high"
	default:
		return ""
	}
}

func toMistralMsgs(msgs []Msg) []mistralMsg {
	var out []mistralMsg
	// toolOK tracks whether a tool message may legally follow at this point —
	// true only directly after an assistant with tool_calls, or another tool
	// message. Orphan tool messages (e.g. from a legacy/edited session) are
	// downgraded to user text so Mistral's role ordering stays valid.
	toolOK := false
	for _, msg := range msgs {
		if msg.Role == Assistant && len(msg.ToolCalls) == 0 && len(msg.Imgs) == 0 && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		switch {
		case msg.Role == Tool:
			if toolOK {
				out = append(out, mistralMsg{
					Role:       "tool",
					ToolCallID: msg.ToolCallID,
					Name:       msg.ToolName,
					Content:    msg.Content,
				})
			} else {
				block := fmt.Sprintf("<tool_output name=%q>\n%s\n</tool_output>\n", msg.ToolName, msg.Content)
				if n := len(out); n > 0 && out[n-1].Role == "user" {
					if s, ok := out[n-1].Content.(string); ok {
						out[n-1].Content = s + block
					} else {
						out = append(out, mistralMsg{Role: "user", Content: toolResultsPreamble + block})
					}
				} else {
					out = append(out, mistralMsg{Role: "user", Content: toolResultsPreamble + block})
				}
				toolOK = false
			}
			if len(msg.Imgs) > 0 {
				out = append(out, mistralMsg{Role: "user", Content: mistralImageBlocks(msg.Imgs, "")})
				toolOK = false
			}
		case msg.Role == Assistant && len(msg.ToolCalls) > 0:
			var content interface{}
			if msg.Content != "" {
				content = msg.Content
			}
			out = append(out, mistralMsg{
				Role:      "assistant",
				Content:   content,
				ToolCalls: toMistralToolCalls(msg.ToolCalls),
			})
			toolOK = true
		case len(msg.Imgs) > 0:
			out = append(out, mistralMsg{Role: string(msg.Role), Content: mistralImageBlocks(msg.Imgs, msg.Content)})
			toolOK = false
		default:
			out = append(out, mistralMsg{Role: string(msg.Role), Content: msg.Content})
			toolOK = false
		}
	}
	return out
}

func mistralImageBlocks(imgs []Img, text string) []map[string]interface{} {
	var blocks []map[string]interface{}
	for _, img := range imgs {
		blocks = append(blocks, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": "data:" + img.Mime + ";base64," + img.Data,
			},
		})
	}
	if text != "" {
		blocks = append(blocks, map[string]interface{}{"type": "text", "text": text})
	}
	return blocks
}

func toMistralToolCalls(calls []ToolCall) []mistralReqToolCall {
	out := make([]mistralReqToolCall, 0, len(calls))
	for _, tc := range calls {
		argsJSON, err := json.Marshal(tc.Args)
		if err != nil {
			argsJSON = []byte("{}")
		}
		out = append(out, mistralReqToolCall{
			Id:       tc.ID,
			Type:     "function",
			Function: mistralReqFunc{Name: tc.Name, Arguments: string(argsJSON)},
		})
	}
	return out
}

const toolIDAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Mistral requires tool_call ids to be exactly 9 alphanumeric characters.
func ensureToolID(id string) string {
	if validToolID(id) {
		return id
	}
	buf := make([]byte, 9)
	if _, err := rand.Read(buf); err != nil {
		return "toolcall0"
	}
	for i := range buf {
		buf[i] = toolIDAlphabet[int(buf[i])%len(toolIDAlphabet)]
	}
	return string(buf)
}

func validToolID(id string) bool {
	if len(id) != 9 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func (m *mistral) ChatStream(ctx context.Context, msgs []Msg, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	reqBody := mistralReq{
		Model:           m.model,
		Msgs:            toMistralMsgs(msgs),
		Stream:          true,
		PromptCacheKey:  m.cacheKey,
		ReasoningEffort: mistralReasoningEffort(m.effort),
	}
	for _, t := range tools {
		reqBody.Tools = append(reqBody.Tools, mistralTool{
			Type: "function",
			Function: mistralFunc{
				Name:        t.Name,
				Description: t.Description,
				Params:      t.Params,
			},
		})
	}

	log, _ := os.OpenFile(filepath.Join("/Users/flipster/.koko/", "koko.debug"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	debugLog := slog.New(slog.NewJSONHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	bodyJSON, err := json.Marshal(reqBody)
	if err == nil {
		debugLog.Info("mistral request body", "body", string(bodyJSON))
	}

	streamCtx, guard := newStallGuard(ctx, streamIdleTimeout)
	defer guard.done()

	resp, err := sendReq(streamCtx, m.client, m.baseURL+"/chat/completions", reqBody, map[string]string{
		"Authorization": "Bearer " + m.apiKey,
		"Accept":        "text/event-stream",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type toolAcc struct {
		id   string
		name string
		args strings.Builder
	}

	var content strings.Builder
	var usage Usg
	var finishReason string
	var toolAccs []*toolAcc
	byIndex := make(map[int]*toolAcc)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		guard.progress()
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk mistralStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			if chunk.Choices[0].FinishReason != "" {
				finishReason = chunk.Choices[0].FinishReason
			}
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				content.WriteString(delta.Content)
				if onDelta != nil {
					onDelta(StreamDelta{Text: delta.Content})
				}
			}
			for _, tc := range delta.ToolCalls {
				acc := byIndex[tc.Index]
				if tc.Function.Name != "" || acc == nil {
					acc = &toolAcc{id: tc.Id, name: tc.Function.Name}
					toolAccs = append(toolAccs, acc)
					byIndex[tc.Index] = acc
				}
				if tc.Id != "" {
					acc.id = tc.Id
				}
				if tc.Function.Arguments != "" {
					acc.args.WriteString(tc.Function.Arguments)
				}
			}
		}
		if chunk.Usg.PromptTokens > 0 || chunk.Usg.CompletionTokens > 0 {
			usage = Usg{
				InputTokens:  chunk.Usg.PromptTokens,
				OutputTokens: chunk.Usg.CompletionTokens,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stream: %w", streamErr(streamCtx, err))
	}

	result := &Response{Content: content.String(), Usage: usage, StopReason: finishReason}

	for _, acc := range toolAccs {
		if acc.name == "" {
			continue
		}
		parsed := map[string]interface{}{}
		rawArgs := strings.TrimSpace(acc.args.String())
		if rawArgs != "" {
			if err := json.Unmarshal([]byte(rawArgs), &parsed); err != nil {
				slog.Warn("mistral stream tool args parse failed", "tool", acc.name, "raw_len", len(rawArgs), "err", err)
				continue
			}
		}
		args := coerceArgs(parsed)
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:   ensureToolID(acc.id),
			Name: acc.name,
			Args: args,
		})
	}

	if onDelta != nil {
		onDelta(StreamDelta{Done: true, Response: result})
	}
	return result, nil
}

type mistralMsg struct {
	Role       string               `json:"role"`
	Content    interface{}          `json:"content"`
	ToolCalls  []mistralReqToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	Name       string               `json:"name,omitempty"`
}

type mistralReqToolCall struct {
	Id       string         `json:"id"`
	Type     string         `json:"type"`
	Function mistralReqFunc `json:"function"`
}

type mistralReqFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type mistralFunc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Params      Schema `json:"parameters"`
}

type mistralTool struct {
	Type     string      `json:"type"`
	Function mistralFunc `json:"function"`
}

type mistralReq struct {
	Model           string        `json:"model"`
	Msgs            []mistralMsg  `json:"messages"`
	Tools           []mistralTool `json:"tools,omitempty"`
	Stream          bool          `json:"stream,omitempty"`
	PromptCacheKey  string        `json:"prompt_cache_key,omitempty"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
}

type mistralStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string            `json:"content"`
			ToolCalls []mistralToolCall `json:"tool_calls,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usg mistralUsg `json:"usage"`
}

type mistralToolCall struct {
	Index    int    `json:"index"`
	Id       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type mistralUsg struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
