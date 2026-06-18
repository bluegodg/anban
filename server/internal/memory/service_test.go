package memory

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/llm"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func newTestMemoryService(t *testing.T, xc xiaozhiclient.Client, extractor llm.FactExtractor, syncer PromptSyncer) (*Service, *Store) {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	memoryStore := NewStore(st.DB)
	if err := memoryStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return NewService(memoryStore, xc, extractor, syncer, Options{HistoryLimit: 20, MaxFacts: 5}), memoryStore
}

func TestServiceDistillDeviceExtractsDedupesAndSyncsPrompt(t *testing.T) {
	ctx := context.Background()
	xc := &memoryHistoryClient{history: []xiaozhiclient.HistoryMessage{
		{Role: "user", Text: "我早上还是想喝豆浆。", At: time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)},
		{Role: "assistant", Text: "好，我记住您喜欢早餐喝豆浆。", At: time.Date(2026, 6, 16, 8, 0, 2, 0, time.UTC)},
	}}
	extractor := &fakeFactExtractor{facts: []string{
		"老人最近喜欢早餐喝豆浆。",
		"老人最近喜欢早餐喝豆浆。",
		"老人腰酸时想先坐一会儿再散步。",
	}}
	syncer := &fakePromptSyncer{}
	svc, memoryStore := newTestMemoryService(t, xc, extractor, syncer)

	result, err := svc.DistillDevice(ctx, " dev-001 ")
	if err != nil {
		t.Fatalf("DistillDevice: %v", err)
	}
	if result.Degraded {
		t.Fatalf("result.Degraded = true, want false: %+v", result)
	}
	if result.AddedFacts != 2 {
		t.Fatalf("AddedFacts = %d, want 2", result.AddedFacts)
	}
	if xc.gotLimit != 20 || extractor.got.DeviceID != "dev-001" || len(extractor.got.Messages) != 2 {
		t.Fatalf("history/extractor call mismatch: limit=%d req=%+v", xc.gotLimit, extractor.got)
	}

	facts, err := memoryStore.ListFacts(ctx, "dev-001", 10)
	if err != nil {
		t.Fatalf("ListFacts: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("facts = %+v, want 2 deduped facts", facts)
	}
	if syncer.deviceID != "dev-001" || len(syncer.facts) != 2 {
		t.Fatalf("prompt sync = %q/%+v, want two facts for dev-001", syncer.deviceID, syncer.facts)
	}
}

func TestServiceDistillDeviceWithoutExtractorDegradesWithoutHistoryCall(t *testing.T) {
	ctx := context.Background()
	xc := &memoryHistoryClient{}
	syncer := &fakePromptSyncer{}
	svc, _ := newTestMemoryService(t, xc, nil, syncer)

	result, err := svc.DistillDevice(ctx, "dev-001")
	if err != nil {
		t.Fatalf("DistillDevice without extractor: %v", err)
	}
	if !result.Degraded {
		t.Fatalf("result.Degraded = false, want graceful LLM-disabled degradation")
	}
	if xc.historyCalls != 0 {
		t.Fatalf("GetHistory calls = %d, want 0 when LLM is disabled", xc.historyCalls)
	}
	if syncer.called {
		t.Fatalf("prompt sync called with no extractor; want profile-only fallback to stay in profile domain")
	}
}

func TestServiceManualFactsCanBeListedEditedDeletedAndSynced(t *testing.T) {
	ctx := context.Background()
	syncer := &fakePromptSyncer{}
	svc, _ := newTestMemoryService(t, &memoryHistoryClient{}, nil, syncer)

	fact, err := svc.AddManualFact(ctx, FactRequest{DeviceID: " dev-001 ", Text: " 蓝喜欢早饭后晒太阳。 "})
	if err != nil {
		t.Fatalf("AddManualFact: %v", err)
	}
	if fact.DeviceID != "dev-001" || fact.Source != "manual" || fact.Text != "蓝喜欢早饭后晒太阳。" {
		t.Fatalf("fact = %+v, want normalized manual fact", fact)
	}
	if syncer.deviceID != "dev-001" || len(syncer.facts) != 1 || syncer.facts[0] != "蓝喜欢早饭后晒太阳。" {
		t.Fatalf("prompt sync = %q/%+v, want one manual fact", syncer.deviceID, syncer.facts)
	}

	got, err := svc.ListFacts(ctx, "dev-001", 10)
	if err != nil {
		t.Fatalf("ListFacts: %v", err)
	}
	if len(got) != 1 || got[0].ID != fact.ID {
		t.Fatalf("facts = %+v, want added fact", got)
	}

	updated, err := svc.UpdateFact(ctx, "dev-001", fact.ID, FactRequest{Text: "蓝喜欢傍晚给花浇水。"})
	if err != nil {
		t.Fatalf("UpdateFact: %v", err)
	}
	if updated.Text != "蓝喜欢傍晚给花浇水。" {
		t.Fatalf("updated text = %q", updated.Text)
	}
	if len(syncer.facts) != 1 || syncer.facts[0] != "蓝喜欢傍晚给花浇水。" {
		t.Fatalf("prompt sync after update = %+v", syncer.facts)
	}

	if err := svc.DeleteFact(ctx, "dev-001", fact.ID); err != nil {
		t.Fatalf("DeleteFact: %v", err)
	}
	if syncer.deviceID != "dev-001" || len(syncer.facts) != 0 {
		t.Fatalf("prompt sync after delete = %q/%+v, want empty fact list for dev-001", syncer.deviceID, syncer.facts)
	}
}

type fakeFactExtractor struct {
	facts []string
	got   llm.FactExtractionRequest
}

func (f *fakeFactExtractor) ExtractFacts(ctx context.Context, req llm.FactExtractionRequest) ([]string, error) {
	f.got = req
	return append([]string(nil), f.facts...), nil
}

type fakePromptSyncer struct {
	called   bool
	deviceID string
	facts    []string
}

func (f *fakePromptSyncer) SyncMemoryFacts(ctx context.Context, deviceID string, facts []string) error {
	f.called = true
	f.deviceID = deviceID
	f.facts = append([]string(nil), facts...)
	return nil
}

type memoryHistoryClient struct {
	xiaozhiclient.FakeClient
	history      []xiaozhiclient.HistoryMessage
	historyCalls int
	gotLimit     int
}

func (c *memoryHistoryClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]xiaozhiclient.HistoryMessage, error) {
	c.historyCalls++
	c.gotLimit = limit
	return append([]xiaozhiclient.HistoryMessage(nil), c.history...), nil
}

func (c *memoryHistoryClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
