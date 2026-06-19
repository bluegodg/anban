package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ArkConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type ArkClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewArkClient(cfg ArkConfig) *ArkClient {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &ArkClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:     strings.TrimSpace(cfg.APIKey),
		model:      strings.TrimSpace(cfg.Model),
		httpClient: client,
	}
}

func (c *ArkClient) ExtractFacts(ctx context.Context, req FactExtractionRequest) ([]string, error) {
	if c == nil || c.baseURL == "" || c.apiKey == "" || c.model == "" {
		return nil, fmt.Errorf("llm: ark client is not configured")
	}

	body := arkChatRequest{
		Model: c.model,
		Messages: []arkMessage{
			{Role: "system", Content: factExtractionSystemPrompt(req.Limit)},
			{Role: "user", Content: factExtractionUserPrompt(req)},
		},
		Temperature: 0.1,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("llm: ark status %d: %s", resp.StatusCode, strings.TrimSpace(string(limited)))
	}

	var decoded arkChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Choices) == 0 {
		return nil, nil
	}
	return parseFactList(decoded.Choices[0].Message.Content), nil
}

type arkChatRequest struct {
	Model       string       `json:"model"`
	Messages    []arkMessage `json:"messages"`
	Temperature float64      `json:"temperature"`
}

type arkMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type arkChatResponse struct {
	Choices []struct {
		Message arkMessage `json:"message"`
	} `json:"choices"`
}

func factExtractionSystemPrompt(limit int) string {
	if limit <= 0 {
		limit = 8
	}
	return fmt.Sprintf("你是安伴的记忆沉淀器。只输出 JSON 字符串数组，最多 %d 条。每条必须是稳定、以陪伴对象为中心的长期事实，20-60 字，避免医疗诊断、避免重复。不要记录设备故障、权限或功能状态，不要记录助手自己的行为或推测，不要记录一次性任务或临时状态。若姓名或称呼被纠正，只保留当前正确值；不要记录被否定的旧姓名或旧称呼，也不要记录曾经叫错这件事。", limit)
}

func factExtractionUserPrompt(req FactExtractionRequest) string {
	var b strings.Builder
	if len(req.ExistingFacts) > 0 {
		b.WriteString("已有事实：\n")
		for _, fact := range req.ExistingFacts {
			fact = strings.TrimSpace(fact)
			if fact != "" {
				b.WriteString("- ")
				b.WriteString(fact)
				b.WriteByte('\n')
			}
		}
	}
	b.WriteString("最近对话：\n")
	for _, msg := range req.Messages {
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(msg.Role))
		b.WriteString(": ")
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return b.String()
}

func parseFactList(content string) []string {
	content = stripMarkdownCodeFence(content)
	if values, ok := decodeFactJSON(content); ok {
		return cleanFacts(values)
	}

	lines := strings.Split(content, "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		line = strings.Trim(line, "\"，,。 ")
		if line != "" {
			values = append(values, line)
		}
	}
	return cleanFacts(values)
}

func stripMarkdownCodeFence(content string) string {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func decodeFactJSON(content string) ([]string, bool) {
	var value any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return nil, false
	}
	return extractFactValues(value), true
}

func extractFactValues(value any) []string {
	switch value := value.(type) {
	case string:
		return []string{value}
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			out = append(out, extractFactValues(item)...)
		}
		return out
	case map[string]any:
		for _, key := range []string{"facts", "items", "memories"} {
			if nested, ok := value[key]; ok {
				return extractFactValues(nested)
			}
		}
		for _, key := range []string{"fact", "text", "memory", "content"} {
			if text, ok := value[key].(string); ok {
				return []string{text}
			}
		}
	}
	return nil
}

func cleanFacts(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || isJSONStructureFragment(value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func isJSONStructureFragment(value string) bool {
	trimmed := strings.TrimSpace(value)
	switch trimmed {
	case "[", "]", "{", "}", "```", "```json":
		return true
	}
	trimmed = strings.TrimLeft(trimmed, "\"'\\")
	for _, key := range []string{"fact", "text", "memory", "content"} {
		if strings.HasPrefix(trimmed, key) && strings.Contains(trimmed, `\":`) {
			return true
		}
	}
	return false
}
