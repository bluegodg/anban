package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFactExtractionPromptKeepsCorrectedIdentityOutOfOldAliases(t *testing.T) {
	prompt := factExtractionSystemPrompt(20)
	for _, want := range []string{"姓名或称呼被纠正", "只保留当前正确值", "被否定的旧姓名或旧称呼"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want identity correction rule %q", prompt, want)
		}
	}
}

func TestFactExtractionPromptKeepsOnlyStablePersonFacts(t *testing.T) {
	prompt := factExtractionSystemPrompt(20)
	for _, want := range []string{
		"稳定、以陪伴对象为中心",
		"设备故障、权限或功能状态",
		"助手自己的行为或推测",
		"一次性任务或临时状态",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want memory quality rule %q", prompt, want)
		}
	}
}

func TestParseFactListAcceptsStructuredJSONVariants(t *testing.T) {
	want := []string{"老人喜欢养花。", "老人觉得播报语速偏快。"}
	for name, content := range map[string]string{
		"object array":    `[{"fact":"老人喜欢养花。"},{"text":"老人觉得播报语速偏快。"}]`,
		"fenced envelope": "```json\n{\"facts\":[{\"fact\":\"老人喜欢养花。\"},{\"content\":\"老人觉得播报语速偏快。\"}]}\n```",
	} {
		t.Run(name, func(t *testing.T) {
			if got := parseFactList(content); !equalStrings(got, want) {
				t.Fatalf("parseFactList() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestCleanFactsRejectsJSONStructureFragments(t *testing.T) {
	got := cleanFacts([]string{
		"[",
		"]",
		"{",
		"}",
		`fact\": \"老人喜欢养花`,
		"老人喜欢养花。",
	})
	want := []string{"老人喜欢养花。"}
	if !equalStrings(got, want) {
		t.Fatalf("cleanFacts() = %#v, want %#v", got, want)
	}
}

func TestArkClientExtractFactsUsesChatCompletionsWithoutRealNetwork(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotModel string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		var req arkChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotModel = req.Model
		if len(req.Messages) != 2 {
			t.Fatalf("messages = %+v, want system+user", req.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"[\"老人喜欢早餐喝豆浆。\",\"老人喜欢早餐喝豆浆。\",\"老人腰酸时想先坐一会儿。\"]"}}]}`))
	}))
	defer server.Close()

	client := NewArkClient(ArkConfig{
		BaseURL: server.URL,
		APIKey:  "ark_key",
		Model:   "doubao-seed",
	})
	facts, err := client.ExtractFacts(context.Background(), FactExtractionRequest{
		DeviceID: "dev-001",
		Messages: []Message{{Role: "user", Text: "我早饭想喝豆浆。"}},
		Limit:    3,
	})
	if err != nil {
		t.Fatalf("ExtractFacts: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer ark_key" || gotModel != "doubao-seed" {
		t.Fatalf("auth/model = %q/%q, want Ark bearer and model", gotAuth, gotModel)
	}
	want := []string{"老人喜欢早餐喝豆浆。", "老人腰酸时想先坐一会儿。"}
	if !equalStrings(facts, want) {
		t.Fatalf("facts = %#v, want %#v", facts, want)
	}
}

func TestArkClientGeneratePortraitUsesProfileMemoryAndCleansResponse(t *testing.T) {
	var gotRequest arkChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"```text\\nAI认知画像：重视家人，喜欢养花，交流时偏好温和直接的表达。\\n```\"}}]}"))
	}))
	defer server.Close()

	client := NewArkClient(ArkConfig{BaseURL: server.URL, APIKey: "secret", Model: "portrait-model"})
	portrait, err := client.GeneratePortrait(context.Background(), PortraitRequest{
		ProfileContext:   "陪伴对象姓名：蓝\n喜好：养花",
		MemoryFacts:      []string{"老人关注世界杯。"},
		PreviousPortrait: "喜欢安静。",
	})
	if err != nil {
		t.Fatalf("GeneratePortrait: %v", err)
	}
	if portrait != "重视家人，喜欢养花，交流时偏好温和直接的表达。" {
		t.Fatalf("portrait = %q, want cleaned plain text", portrait)
	}
	if len(gotRequest.Messages) != 2 || !strings.Contains(gotRequest.Messages[1].Content, "老人关注世界杯") || !strings.Contains(gotRequest.Messages[1].Content, "喜欢安静") {
		t.Fatalf("messages = %+v, want profile, memory, and previous portrait", gotRequest.Messages)
	}
	for _, want := range []string{"不得编造", "稳定特征", "不要输出医疗诊断"} {
		if !strings.Contains(gotRequest.Messages[0].Content, want) {
			t.Fatalf("system prompt = %q, want %q", gotRequest.Messages[0].Content, want)
		}
	}
}

func TestPortraitPromptAndCleanerAvoidSecondPersonAddress(t *testing.T) {
	prompt := portraitSystemPrompt()
	for _, want := range []string{"第三人称", "不要用“你”称呼陪伴对象"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt = %q, want %q", prompt, want)
		}
	}

	got := cleanPortrait("AI认知画像：你名叫蓝，常用称呼为蓝，你的兴趣是养花。")
	for _, bad := range []string{"你名叫", "你的"} {
		if strings.Contains(got, bad) {
			t.Fatalf("portrait = %q, want no second-person marker %q", got, bad)
		}
	}
	if !strings.Contains(got, "陪伴对象名叫蓝") || !strings.Contains(got, "其兴趣是养花") {
		t.Fatalf("portrait = %q, want third-person normalization", got)
	}
}

func TestArkClientExtractFactsReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewArkClient(ArkConfig{BaseURL: server.URL, APIKey: "bad", Model: "doubao-seed"})
	if _, err := client.ExtractFacts(context.Background(), FactExtractionRequest{}); err == nil {
		t.Fatal("expected status error, got nil")
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
