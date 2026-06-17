package profile

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func newTestService(t *testing.T, xc xiaozhiclient.Client) *Service {
	t.Helper()
	svc, _ := newTestServiceWithStore(t, xc)
	return svc
}

func newTestServiceWithStore(t *testing.T, xc xiaozhiclient.Client) (*Service, *Store) {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	profileStore := NewStore(st.DB)
	if err := profileStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return NewService(profileStore, xc), profileStore
}

func TestServiceUpdatePersistsProfileAndSyncsPrompt(t *testing.T) {
	xc := &profileClient{}
	svc := newTestService(t, xc)
	ctx := context.Background()

	got, err := svc.Update(ctx, UpdateRequest{
		DeviceID: " dev-001 ",
		Fields: Fields{
			Name:          "王秀英",
			Nickname:      "妈",
			Children:      []string{"小明", "小红"},
			Grandchildren: []string{"小宝（7岁）"},
			Hobbies:       []string{"豫剧", "下棋"},
			Schedule:      "早 6 点起，晚 9 点睡",
			Health:        "高血压、轻度糖尿病",
			Taboos:        []string{"甜食"},
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.DeviceID != "dev-001" || got.Fields.Name != "王秀英" {
		t.Fatalf("profile = %+v, want trimmed device and stored fields", got)
	}
	if xc.gotDeviceID != "dev-001" {
		t.Fatalf("SetRolePrompt deviceID = %q, want dev-001", xc.gotDeviceID)
	}
	for _, want := range []string{"王秀英", "小宝", "豫剧", "高血压"} {
		if !strings.Contains(xc.gotPrompt, want) {
			t.Fatalf("prompt = %q, want contains %q", xc.gotPrompt, want)
		}
	}

	saved, err := svc.Get(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if saved.ID != got.ID || saved.Prompt == "" {
		t.Fatalf("saved profile = %+v, want persisted profile with prompt", saved)
	}
}

func TestServiceUpdateRejectsMissingDeviceID(t *testing.T) {
	svc := newTestService(t, &profileClient{})

	_, err := svc.Update(context.Background(), UpdateRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceGetRejectsMissingDeviceID(t *testing.T) {
	svc := newTestService(t, &profileClient{})

	_, err := svc.Get(context.Background(), " ")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceUpdateReturnsSyncErrorAfterPersisting(t *testing.T) {
	xc := &profileClient{err: errors.New("manager unavailable")}
	svc := newTestService(t, xc)
	ctx := context.Background()

	got, err := svc.Update(ctx, UpdateRequest{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:     "王秀英",
			Nickname: "妈",
			Hobbies:  []string{"豫剧"},
		},
	})
	if err == nil {
		t.Fatal("expected sync error, got nil")
	}
	if got.DeviceID != "dev-001" || got.ID == 0 {
		t.Fatalf("profile = %+v, want persisted profile returned with error", got)
	}

	saved, getErr := svc.Get(ctx, "dev-001")
	if getErr != nil {
		t.Fatalf("Get after sync error: %v", getErr)
	}
	if saved.Prompt == "" {
		t.Fatal("saved prompt is empty")
	}
}

func TestBuildPromptKeepsPromptWithinPRDBudget(t *testing.T) {
	longText := strings.Repeat("今天腰有点酸但还想去公园散步，", 140)

	prompt := BuildPrompt(Fields{
		Name:          "王秀英",
		Nickname:      "妈",
		Children:      []string{"小明", longText},
		Grandchildren: []string{"小宝", longText},
		Hobbies:       []string{"豫剧", longText},
		Schedule:      longText,
		Health:        longText,
		Taboos:        []string{"甜食", longText},
	})

	if got := len([]rune(prompt)); got > 1500 {
		t.Fatalf("prompt length = %d runes, want <= 1500", got)
	}
	for _, want := range []string{"王秀英", "常用称呼：妈", "小明", "小宝", "豫剧"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want preserve high-value field %q", prompt, want)
		}
	}
}

func TestBuildPromptWithFieldsOnlyMatchesLegacyPromptExactly(t *testing.T) {
	fields := Fields{
		Name:          "王秀英",
		Nickname:      "妈",
		Children:      []string{"小明", "小红"},
		Grandchildren: []string{"小宝（7岁）"},
		Hobbies:       []string{"豫剧", "下棋"},
		Schedule:      "早 6 点起，晚 9 点睡",
		Health:        "高血压、轻度糖尿病",
		Taboos:        []string{"甜食"},
	}
	want := strings.Join([]string{
		"你是安伴，一位温和、耐心、像家人一样陪伴老人的语音助手。",
		"请优先使用下面的家庭画像理解老人，不要生硬复述画像内容，回答要自然、简短、关心当下。",
		"老人问到子女或孙辈姓名、称呼、喜好、健康或忌口时，直接依据家庭画像回答名字或事实；不知道再说明。",
		"当前会话中老人刚说过的事也要当作短期上下文，后续回答要自然承接，不要像第一次听到一样重复追问。",
		"非老人明确要求，不要更改设备设置/音量/屏幕主题/字体；日常陪伴中不要主动调用设备设置工具。",
		"老人本名：王秀英",
		"常用称呼：妈",
		"子女：小明、小红",
		"孙辈：小宝（7岁）",
		"喜好：豫剧、下棋",
		"作息：早 6 点起，晚 9 点睡",
		"健康背景：高血压、轻度糖尿病",
		"忌口和禁忌：甜食",
	}, "\n")

	if got := BuildPromptWith(fields, nil, ""); got != want {
		t.Fatalf("BuildPromptWith fields-only changed legacy prompt:\n got %q\nwant %q", got, want)
	}
	if got := BuildPrompt(fields); got != want {
		t.Fatalf("BuildPrompt changed legacy prompt:\n got %q\nwant %q", got, want)
	}
}

func TestBuildPromptWithMemoryFactsKeepsPromptWithinPRDBudget(t *testing.T) {
	longFact := strings.Repeat("最近午饭后会在阳台晒太阳，", 120)

	prompt := BuildPromptWithMemory(Fields{
		Name:     "王秀英",
		Nickname: "妈",
		Hobbies:  []string{"豫剧"},
	}, []string{
		"老人最近喜欢早餐喝豆浆。",
		"老人说腰酸时想先坐一会儿再散步。",
		longFact,
	})

	if got := len([]rune(prompt)); got > 1500 {
		t.Fatalf("prompt length = %d runes, want <= 1500", got)
	}
	for _, want := range []string{"近期记忆", "早餐喝豆浆", "腰酸时想先坐一会儿", "王秀英"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want contains memory/profile fact %q", prompt, want)
		}
	}
}

func TestBuildPromptWithAllBlocksKeepsPromptWithinBudgetAndTruncatesMindContextFirst(t *testing.T) {
	longMindContext := strings.Repeat("最近很挂念老人午后精神和睡眠，回答时更关切但别唠叨。", 120)
	prompt := BuildPromptWith(Fields{
		Name:          "王秀英",
		Grandchildren: []string{"小宝"},
		Hobbies:       []string{"豫剧"},
	}, []string{
		"老人最近喜欢早餐喝豆浆。",
		"老人说腰酸时想先坐一会儿再散步。",
	}, longMindContext)

	if got := len([]rune(prompt)); got > 1500 {
		t.Fatalf("prompt length = %d runes, want <= 1500", got)
	}
	for _, want := range []string{"王秀英", "小宝", "近期记忆：老人最近喜欢早餐喝豆浆", "心境："} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want contains %q", prompt, want)
		}
	}
	if !strings.Contains(prompt, "...") {
		t.Fatalf("prompt = %q, want long mind context truncated first", prompt)
	}
}

func TestServiceUpdatePreservesMemoryFactsAndMindContext(t *testing.T) {
	xc := &profileClient{}
	svc, profileStore := newTestServiceWithStore(t, xc)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &Profile{
		DeviceID:    "dev-001",
		Fields:      Fields{Name: "旧名字"},
		MemoryFacts: []string{"老人最近喜欢早餐喝豆浆。"},
		MindContext: "最近你较挂念老人，语气更关切些。",
		Prompt:      "old prompt",
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	got, err := svc.Update(ctx, UpdateRequest{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:          "王秀英",
			Grandchildren: []string{"小宝"},
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(got.MemoryFacts) != 1 || got.MindContext == "" {
		t.Fatalf("profile = %+v, want preserved memory facts and mind context", got)
	}
	for _, want := range []string{"王秀英", "小宝", "早餐喝豆浆", "心境：最近你较挂念老人"} {
		if !strings.Contains(xc.gotPrompt, want) {
			t.Fatalf("prompt = %q, want contains %q", xc.gotPrompt, want)
		}
	}
}

func TestServiceSyncMemoryFactsPreservesFieldsAndMindContext(t *testing.T) {
	xc := &profileClient{}
	svc, profileStore := newTestServiceWithStore(t, xc)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &Profile{
		DeviceID:    "dev-001",
		Fields:      Fields{Name: "王秀英", Grandchildren: []string{"小宝"}},
		MindContext: "最近你较挂念老人，语气更关切些。",
		Prompt:      "old prompt",
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	if err := svc.SyncMemoryFacts(ctx, "dev-001", []string{"老人最近喜欢早餐喝豆浆。"}); err != nil {
		t.Fatalf("SyncMemoryFacts: %v", err)
	}
	saved, err := svc.Get(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(saved.MemoryFacts) != 1 || saved.MindContext == "" {
		t.Fatalf("saved profile = %+v, want memory facts and preserved mind context", saved)
	}
	for _, want := range []string{"王秀英", "小宝", "早餐喝豆浆", "心境：最近你较挂念老人"} {
		if !strings.Contains(xc.gotPrompt, want) {
			t.Fatalf("prompt = %q, want contains %q", xc.gotPrompt, want)
		}
	}
}

func TestBuildPromptGuidesFamilyProfileRecall(t *testing.T) {
	prompt := BuildPrompt(Fields{
		Name:          "王秀英",
		Nickname:      "妈",
		Children:      []string{"小明"},
		Grandchildren: []string{"小宝（7岁）"},
		Hobbies:       []string{"豫剧"},
		Health:        "高血压",
	})

	for _, want := range []string{
		"问到子女或孙辈姓名",
		"直接依据家庭画像回答名字",
		"不知道再说明",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want recall guidance %q", prompt, want)
		}
	}
}

func TestBuildPromptGuidesCurrentConversationContinuity(t *testing.T) {
	prompt := BuildPrompt(Fields{Nickname: "妈"})

	for _, want := range []string{
		"当前会话",
		"老人刚说过的事",
		"后续回答要自然承接",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want current conversation continuity guidance %q", prompt, want)
		}
	}
}

func TestBuildPromptGuardsDeviceSettingsUnlessElderAsks(t *testing.T) {
	prompt := BuildPrompt(Fields{Nickname: "妈"})

	want := "非老人明确要求，不要更改设备设置/音量/屏幕主题/字体"
	if !strings.Contains(prompt, want) {
		t.Fatalf("prompt = %q, want device settings guard %q", prompt, want)
	}
}

type profileClient struct {
	xiaozhiclient.FakeClient
	gotDeviceID string
	gotPrompt   string
	err         error
}

func (c *profileClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	c.gotDeviceID = deviceID
	c.gotPrompt = prompt
	return c.err
}

func (c *profileClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
