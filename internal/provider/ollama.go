package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/meeseeks/koko/internal/httputil"
)

type OllamaProvider struct {
	model   string
	baseURL string
	client  *http.Client
}

func NewOllama(model, baseURL string) (*OllamaProvider, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3"
	}
	return &OllamaProvider{
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{},
	}, nil
}

func (o *OllamaProvider) Name() string      { return "ollama" }
func (o *OllamaProvider) Model() string     { return o.model }
func (o *OllamaProvider) SetModel(m string) { o.model = m }

func toOllamaMessages(messages []Message) []ollamaMessage {
	var out []ollamaMessage
	for _, m := range messages {
		msg := ollamaMessage{Role: string(m.Role), Content: m.Content}
		for _, img := range m.Images {
			msg.Images = append(msg.Images, img.Data)
		}
		out = append(out, msg)
	}
	return out
}

func (o *OllamaProvider) ChatStream(ctx context.Context, messages []Message, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	ollamaMessages := toOllamaMessages(messages)

	reqBody := ollamaRequest{
		Model:    o.model,
		Messages: ollamaMessages,
		Stream:   true,
	}
	if len(tools) > 0 {
		reqBody.Tools = convertTools(tools)
		reqBody.Stream = false
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	resp, err := httputil.WithRetry(ctx, o.client, req, 5)
	if err != nil {
		return nil, fmt.Errorf("sending request (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama error (status %d): %s", resp.StatusCode, sanitizeErrorBody(body, 512))
	}

	if len(tools) > 0 {
		return o.parseNonStreamingResponse(resp.Body, onDelta)
	}

	return o.parseStreamingResponse(resp.Body, onDelta)
}

func (o *OllamaProvider) parseStreamingResponse(body io.Reader, onDelta func(StreamDelta)) (*Response, error) {
	var content strings.Builder
	var usage Usage
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk ollamaResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if onDelta != nil {
				onDelta(StreamDelta{Text: chunk.Message.Content})
			}
		}
		if chunk.Done {
			usage = Usage{
				InputTokens:  chunk.PromptEvalCount,
				OutputTokens: chunk.EvalCount,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stream: %w", err)
	}

	result := &Response{Content: content.String(), Usage: usage}
	if onDelta != nil {
		onDelta(StreamDelta{Done: true, Response: result})
	}
	return result, nil
}

func (o *OllamaProvider) parseNonStreamingResponse(body io.Reader, onDelta func(StreamDelta)) (*Response, error) {
	respBody, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &Response{
		Content: ollamaResp.Message.Content,
		Usage: Usage{
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
		},
	}

	if ollamaResp.Message.Content != "" && onDelta != nil {
		onDelta(StreamDelta{Text: ollamaResp.Message.Content})
	}

	for _, tc := range ollamaResp.Message.ToolCalls {
		args := make(map[string]string, len(tc.Function.Arguments))
		for k, v := range tc.Function.Arguments {
			switch vv := v.(type) {
			case string:
				args[k] = vv
			case float64:
				args[k] = strconv.FormatFloat(vv, 'f', -1, 64)
			case bool:
				args[k] = strconv.FormatBool(vv)
			default:
				b, _ := json.Marshal(vv)
				args[k] = string(b)
			}
		}
		slog.Info("ollama tool call", "tool", tc.Function.Name, "arg_keys", keysOf(args))
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			Name: tc.Function.Name,
			Args: args,
		})
	}

	if onDelta != nil {
		onDelta(StreamDelta{Done: true, Response: result})
	}
	return result, nil
}

func (o *OllamaProvider) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	ollamaMessages := toOllamaMessages(messages)

	reqBody := ollamaRequest{
		Model:    o.model,
		Messages: ollamaMessages,
		Stream:   false,
	}
	if len(tools) > 0 {
		reqBody.Tools = convertTools(tools)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	resp, err := httputil.WithRetry(ctx, o.client, req, 5)
	if err != nil {
		return nil, fmt.Errorf("sending request (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama error (status %d): %s", resp.StatusCode, sanitizeErrorBody(respBody, 512))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	result := &Response{
		Content: ollamaResp.Message.Content,
		Usage: Usage{
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
		},
	}

	for _, tc := range ollamaResp.Message.ToolCalls {
		args := make(map[string]string, len(tc.Function.Arguments))
		for k, v := range tc.Function.Arguments {
			switch vv := v.(type) {
			case string:
				args[k] = vv
			case float64:
				args[k] = strconv.FormatFloat(vv, 'f', -1, 64)
			case bool:
				args[k] = strconv.FormatBool(vv)
			default:
				b, _ := json.Marshal(vv)
				args[k] = string(b)
			}
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return result, nil
}

func convertTools(tools []ToolDef) []ollamaTool {
	out := make([]ollamaTool, len(tools))
	for i, t := range tools {
		out[i] = ollamaTool{
			Type: "function",
			Function: ollamaFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return out
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ollamaTool struct {
	Type     string         `json:"type"`
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
}

type ollamaResponse struct {
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}
