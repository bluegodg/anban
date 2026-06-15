package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
