package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/meeseeks/koko/httputil"
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

func (o *OllamaProvider) ChatStream(ctx context.Context, messages []Message, tools []ToolDef, onDelta func(StreamDelta)) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var ollamaMessages []ollamaMessage
	for _, m := range messages {
		ollamaMessages = append(ollamaMessages, ollamaMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	reqBody := ollamaRequest{
		Model:    o.model,
		Messages: ollamaMessages,
		Stream:   true,
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

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama error (status %d): %s", resp.StatusCode, sanitizeErrorBody(body, 512))
	}

	var content strings.Builder
	var usage Usage
	scanner := bufio.NewScanner(resp.Body)
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

func (o *OllamaProvider) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var ollamaMessages []ollamaMessage
	for _, m := range messages {
		ollamaMessages = append(ollamaMessages, ollamaMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	reqBody := ollamaRequest{
		Model:    o.model,
		Messages: ollamaMessages,
		Stream:   false,
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

	resp, err := httputil.DoWithRetry(ctx, o.client, req, 3)
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

	return &Response{
		Content: ollamaResp.Message.Content,
		Usage: Usage{
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
		},
	}, nil
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaResponse struct {
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}
