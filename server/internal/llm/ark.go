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
	content, err := c.chatCompletion(ctx, []arkMessage{
		{Role: "system", Content: factExtractionSystemPrompt(req.Limit)},
		{Role: "user", Content: factExtractionUserPrompt(req)},
	}, 0.1)
	if err != nil {
		return nil, err
	}
	return parseFactList(content), nil
}

func (c *ArkClient) GeneratePortrait(ctx context.Context, req PortraitRequest) (string, error) {
	content, err := c.chatCompletion(ctx, []arkMessage{
		{Role: "system", Content: portraitSystemPrompt()},
		{Role: "user", Content: portraitUserPrompt(req)},
	}, 0.2)
	if err != nil {
		return "", err
	}
	return cleanPortrait(content), nil
}

func (c *ArkClient) chatCompletion(ctx context.Context, messages []arkMessage, temperature float64) (string, error) {
	if c == nil || c.baseURL == "" || c.apiKey == "" || c.model == "" {
		return "", fmt.Errorf("llm: ark client is not configured")
	}

	body := arkChatRequest{Model: c.model, Messages: messages, Temperature: temperature}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("llm: ark status %d: %s", resp.StatusCode, strings.TrimSpace(string(limited)))
	}

	var decoded arkChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", nil
	}
	return decoded.Choices[0].Message.Content, nil
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

func portraitSystemPrompt() string {
	return "你是安伴的陪伴对象认知画像整理器。根据管理员资料和稳定长期记忆，提炼 60-180 字的中文画像。必须用第三人称描述陪伴对象，可以写“陪伴对象/老人/她/他”，不要用“你”称呼陪伴对象，避免让下游模型误解成助手自我设定。只写有依据的稳定特征，不得编造身份、经历、关系或健康信息；不要输出医疗诊断，不记录设备故障、一次性任务或助手行为。保留管理员已确认画像中的有效事实，但新证据冲突时以当前资料和记忆为准。只输出画像正文，不加标题、标签、Markdown 或 JSON。"
}

func portraitUserPrompt(req PortraitRequest) string {
	var b strings.Builder
	b.WriteString("管理员资料：\n")
	if value := strings.TrimSpace(req.ProfileContext); value != "" {
		b.WriteString(value)
		b.WriteByte('\n')
	} else {
		b.WriteString("暂无\n")
	}
	if value := strings.TrimSpace(req.PreviousPortrait); value != "" {
		b.WriteString("上一版画像：\n")
		b.WriteString(value)
		b.WriteByte('\n')
	}
	b.WriteString("专属记忆：\n")
	if len(req.MemoryFacts) == 0 {
		b.WriteString("暂无\n")
	} else {
		for _, fact := range req.MemoryFacts {
			if fact = strings.TrimSpace(fact); fact != "" {
				b.WriteString("- ")
				b.WriteString(fact)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func cleanPortrait(content string) string {
	content = stripMarkdownCodeFence(content)
	content = strings.TrimSpace(strings.Trim(content, "\"'“”"))
	for _, prefix := range []string{"AI认知画像：", "AI画像：", "认知画像："} {
		content = strings.TrimSpace(strings.TrimPrefix(content, prefix))
	}
	content = normalizePortraitPerson(content)
	runes := []rune(content)
	if len(runes) > 360 {
		content = string(runes[:360])
	}
	return content
}

func normalizePortraitPerson(content string) string {
	replacements := []struct {
		old string
		new string
	}{
		{"你名叫", "陪伴对象名叫"},
		{"你叫", "陪伴对象叫"},
		{"您的", "其"},
		{"你的", "其"},
		{"您", "陪伴对象"},
	}
	for _, replacement := range replacements {
		content = strings.ReplaceAll(content, replacement.old, replacement.new)
	}
	return content
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
