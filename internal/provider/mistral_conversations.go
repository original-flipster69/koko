package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

func (m *mistral) chatConversation(ctx context.Context, msgs []Msg, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	var system string
	var nonSystem []Msg
	for _, msg := range msgs {
		if msg.Role == System {
			system = msg.Content
			continue
		}
		nonSystem = append(nonSystem, msg)
	}
	if len(nonSystem) == 0 {
		return nil, fmt.Errorf("no messages to send")
	}

	appendMode := m.convID != "" && hasMsgPrefix(nonSystem, m.committed)
	var toSend []Msg
	if appendMode {
		toSend = nonSystem[len(m.committed):]
	}
	if len(toSend) == 0 {
		appendMode = false
		m.resetConversation()
		toSend = nonSystem
	}

	var resp *Response
	var err error
	if appendMode {
		resp, err = m.convAppend(ctx, toConvInputs(toSend))
	} else {
		resp, err = m.convCreate(ctx, system, toConvInputs(toSend), tools)
	}
	if err != nil {
		m.resetConversation()
		return nil, err
	}

	m.committed = append(nonSystem, Msg{Role: Assistant, Content: assistantPlaceholder(resp)})

	if onDelta != nil {
		if resp.Content != "" {
			onDelta(StreamDelta{Text: resp.Content})
		}
		onDelta(StreamDelta{Done: true, Response: resp})
	}
	return resp, nil
}

func hasMsgPrefix(full, prefix []Msg) bool {
	if len(prefix) > len(full) {
		return false
	}
	for i := range prefix {
		if full[i].Role != prefix[i].Role || full[i].Content != prefix[i].Content {
			return false
		}
	}
	return true
}

func assistantPlaceholder(resp *Response) string {
	if resp.Content != "" {
		return resp.Content
	}
	if len(resp.ToolCalls) > 0 {
		names := make([]string, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			names[i] = tc.Name
		}
		return "[calling tools: " + strings.Join(names, ", ") + "]"
	}
	return ""
}

func toConvInputs(msgs []Msg) []convInput {
	out := make([]convInput, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, convInput{
			Type:    "message.input",
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}
	return out
}

func (m *mistral) headers() map[string]string {
	return map[string]string{"Authorization": "Bearer " + m.apiKey}
}

func (m *mistral) convCreate(ctx context.Context, system string, inputs []convInput, tools []ToolDef) (*Response, error) {
	body := convCreateReq{
		Model:        m.model,
		Instructions: system,
		Inputs:       inputs,
		Store:        true,
		Stream:       false,
	}
	for _, t := range tools {
		body.Tools = append(body.Tools, mistralTool{
			Type:     "function",
			Function: mistralFunc{Name: t.Name, Description: t.Description, Params: t.Params},
		})
	}
	resp, err := sendReq(ctx, m.client, m.baseURL+"/conversations", body, m.headers())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	cr, err := parseConvResp(resp.Body)
	if err != nil {
		return nil, err
	}
	m.convID = cr.ConversationID
	return cr.toResponse(), nil
}

func (m *mistral) convAppend(ctx context.Context, inputs []convInput) (*Response, error) {
	body := convAppendReq{Inputs: inputs, Store: true, Stream: false}
	resp, err := sendReq(ctx, m.client, m.baseURL+"/conversations/"+m.convID, body, m.headers())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	cr, err := parseConvResp(resp.Body)
	if err != nil {
		return nil, err
	}
	return cr.toResponse(), nil
}

func parseConvResp(body io.Reader) (*convResp, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	var cr convResp
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("parsing conversation response: %w", err)
	}
	return &cr, nil
}

func (cr *convResp) toResponse() *Response {
	result := &Response{
		Usage: Usg{
			InputTokens:  cr.Usage.PromptTokens,
			OutputTokens: cr.Usage.CompletionTokens,
		},
	}
	var content strings.Builder
	for _, o := range cr.Outputs {
		if o.Name != "" || o.Type == "function.call" {
			args := make(map[string]string)
			var raw map[string]interface{}
			if err := json.Unmarshal([]byte(o.Arguments), &raw); err == nil {
				args = coerceArgs(raw)
			} else {
				slog.Warn("mistral conversation tool args parse failed", "tool", o.Name, "err", err)
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{Name: o.Name, Args: args})
			continue
		}
		content.WriteString(o.Content)
	}
	result.Content = content.String()
	return result
}

type convInput struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type convCreateReq struct {
	Model        string        `json:"model"`
	Instructions string        `json:"instructions,omitempty"`
	Inputs       []convInput   `json:"inputs"`
	Tools        []mistralTool `json:"tools,omitempty"`
	Store        bool          `json:"store"`
	Stream       bool          `json:"stream"`
}

type convAppendReq struct {
	Inputs []convInput `json:"inputs"`
	Store  bool        `json:"store"`
	Stream bool        `json:"stream"`
}

type convResp struct {
	ConversationID string       `json:"conversation_id"`
	Outputs        []convOutput `json:"outputs"`
	Usage          struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type convOutput struct {
	Type      string `json:"type"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
