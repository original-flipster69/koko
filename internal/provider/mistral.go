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
	for _, msg := range msgs {
		if len(msg.Imgs) == 0 {
			out = append(out, mistralMsg{Role: string(msg.Role), Content: msg.Content})
			continue
		}
		var blocks []map[string]interface{}
		for _, img := range msg.Imgs {
			blocks = append(blocks, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]string{
					"url": "data:" + img.Mime + ";base64," + img.Data,
				},
			})
		}
		if msg.Content != "" {
			blocks = append(blocks, map[string]interface{}{
				"type": "text",
				"text": msg.Content,
			})
		}
		out = append(out, mistralMsg{Role: string(msg.Role), Content: blocks})
	}
	return out
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
					acc = &toolAcc{name: tc.Function.Name}
					toolAccs = append(toolAccs, acc)
					byIndex[tc.Index] = acc
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
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
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
	Index    int `json:"index"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type mistralUsg struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
