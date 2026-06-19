package profile

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluegodg/anban/server/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, _ := newTestServiceWithStore(t)
	return svc
}

func newTestServiceWithStore(t *testing.T) (*Service, *Store) {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	profileStore := NewStore(st.DB)
	if err := profileStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return NewService(profileStore), profileStore
}

func TestServiceUpdatePersistsProfileContext(t *testing.T) {
	svc := newTestService(t)
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
	for _, want := range []string{"王秀英", "小宝", "豫剧", "高血压"} {
		if !strings.Contains(got.Prompt, want) {
			t.Fatalf("context = %q, want contains %q", got.Prompt, want)
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

func TestServiceUpdateBuildsCompanionContextWithoutStyleInstructions(t *testing.T) {
	svc := newTestService(t)

	got, err := svc.Update(context.Background(), UpdateRequest{
		DeviceID: "dev-001",
		Fields:   Fields{Name: "蓝", Hobbies: []string{"养花"}},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Prompt == "" || !strings.Contains(got.Prompt, "陪伴对象姓名：蓝") {
		t.Fatalf("profile context = %q, want persisted companion context", got.Prompt)
	}
	for _, styleText := range []string{"你是安伴", "设备设置工具"} {
		if strings.Contains(got.Prompt, styleText) {
			t.Fatalf("profile context = %q, want no style text %q", got.Prompt, styleText)
		}
	}
}

func TestServiceUpdateRejectsMissingDeviceID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Update(context.Background(), UpdateRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceGetRejectsMissingDeviceID(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Get(context.Background(), " ")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceUpdateDoesNotDependOnManagerAvailability(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	got, err := svc.Update(ctx, UpdateRequest{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:     "王秀英",
			Nickname: "妈",
			Hobbies:  []string{"豫剧"},
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.DeviceID != "dev-001" || got.ID == 0 {
		t.Fatalf("profile = %+v, want persisted profile returned with error", got)
	}

	saved, getErr := svc.Get(ctx, "dev-001")
	if getErr != nil {
		t.Fatalf("Get after update: %v", getErr)
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
	for _, want := range []string{"陪伴对象姓名：王秀英", "常用称呼：妈", "小明", "小宝", "豫剧"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want preserve high-value field %q", prompt, want)
		}
	}
}

func TestBuildPromptWithFieldsOnlyContainsCompanionDataOnly(t *testing.T) {
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
		"陪伴对象姓名：王秀英",
		"常用称呼：妈",
		"子女：小明、小红",
		"孙辈：小宝（7岁）",
		"喜好：豫剧、下棋",
		"作息：早 6 点起，晚 9 点睡",
		"健康背景：高血压、轻度糖尿病",
		"忌口和禁忌：甜食",
	}, "\n")

	if got := BuildPromptWith(fields, nil, ""); got != want {
		t.Fatalf("BuildPromptWith fields-only context:\n got %q\nwant %q", got, want)
	}
	if got := BuildPrompt(fields); got != want {
		t.Fatalf("BuildPrompt context:\n got %q\nwant %q", got, want)
	}
}

func TestBuildPromptWithIncludesAICognitivePortrait(t *testing.T) {
	prompt := BuildPromptWith(Fields{
		Name:       "蓝",
		AIPortrait: "重视家人，也喜欢通过养花保持生活节奏。",
	}, nil, "")

	if !strings.Contains(prompt, "AI认知画像：重视家人，也喜欢通过养花保持生活节奏。") {
		t.Fatalf("prompt = %q, want AI cognitive portrait", prompt)
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
	for _, want := range []string{"专属记忆", "早餐喝豆浆", "腰酸时想先坐一会儿", "王秀英"} {
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
	for _, want := range []string{"王秀英", "小宝", "专属记忆：老人最近喜欢早餐喝豆浆", "心智上下文："} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want contains %q", prompt, want)
		}
	}
	if !strings.Contains(prompt, "...") {
		t.Fatalf("prompt = %q, want long mind context truncated first", prompt)
	}
}

func TestBuildPromptWithPreservesCompleteNormalMindContext(t *testing.T) {
	mindContext := "最近你较挂念老人(concern 偏高)，语气更关切些；关系温度较暖，回答可以更亲近自然；老人近期偏安静，少追问，先轻声陪着；今天聊过/留意过：空闲循环检测到一段沉默；提醒确认结果进入安伴心智；提醒到期，进入安伴心智观察；老人今天还聊到世界杯和养花；陪伴对象：蓝；画像重点：常用称呼：蓝；记忆重点：老人关注世界杯足球赛事。；老人喜欢养花。"
	if got := len([]rune(mindContext)); got <= maxProfilePromptLineRunes || got > 360 {
		t.Fatalf("test mind context length = %d, want between line limit and 360", got)
	}

	prompt := BuildPromptWith(Fields{Name: "蓝", Nickname: "蓝"}, []string{"老人喜欢养花。"}, mindContext)
	if !strings.Contains(prompt, "记忆重点：老人关注世界杯足球赛事。；老人喜欢养花。") {
		t.Fatalf("prompt = %q, want complete normal mind context", prompt)
	}
}

func TestServiceUpdatePreservesMemoryFactsAndMindContext(t *testing.T) {
	svc, profileStore := newTestServiceWithStore(t)
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
	for _, want := range []string{"王秀英", "小宝", "早餐喝豆浆", "心智上下文：最近你较挂念老人"} {
		if !strings.Contains(got.Prompt, want) {
			t.Fatalf("context = %q, want contains %q", got.Prompt, want)
		}
	}
}

func TestServiceSyncMemoryFactsPreservesFieldsAndMindContext(t *testing.T) {
	svc, profileStore := newTestServiceWithStore(t)
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
	for _, want := range []string{"王秀英", "小宝", "早餐喝豆浆", "心智上下文：最近你较挂念老人"} {
		if !strings.Contains(saved.Prompt, want) {
			t.Fatalf("context = %q, want contains %q", saved.Prompt, want)
		}
	}
}

func TestServiceSyncMindContextPreservesFieldsAndMemoryFacts(t *testing.T) {
	svc, profileStore := newTestServiceWithStore(t)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &Profile{
		DeviceID:    "dev-001",
		Fields:      Fields{Name: "王秀英", Grandchildren: []string{"小宝"}},
		MemoryFacts: []string{"老人最近喜欢早餐喝豆浆。"},
		Prompt:      "old prompt",
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	if err := svc.SyncMindContext(ctx, "dev-001", "最近你较挂念老人，语气更关切些。"); err != nil {
		t.Fatalf("SyncMindContext: %v", err)
	}
	saved, err := svc.Get(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if saved.MindContext == "" || len(saved.MemoryFacts) != 1 {
		t.Fatalf("saved profile = %+v, want mind context and preserved memory facts", saved)
	}
	for _, want := range []string{"王秀英", "小宝", "早餐喝豆浆", "心智上下文：最近你较挂念老人"} {
		if !strings.Contains(saved.Prompt, want) {
			t.Fatalf("context = %q, want contains %q", saved.Prompt, want)
		}
	}
}

func TestServiceSyncMindContextDoesNotGeneratePortrait(t *testing.T) {
	_, profileStore := newTestServiceWithStore(t)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &Profile{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:           "蓝",
			AIPortraitMode: PortraitModeAuto,
		},
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	generator := &fakePortraitGenerator{result: "不应在 Mind 同步中生成"}
	svc := NewService(profileStore, generator)

	if err := svc.SyncMindContext(ctx, "dev-001", "关系温度较暖"); err != nil {
		t.Fatalf("SyncMindContext: %v", err)
	}
	if len(generator.calls) != 0 {
		t.Fatalf("portrait calls = %d, want 0 so startup Mind sync stays local", len(generator.calls))
	}
}

func TestServiceUpdateAutoGeneratesPortraitFromProfileAndMemory(t *testing.T) {
	_, profileStore := newTestServiceWithStore(t)
	generator := &fakePortraitGenerator{result: "重视家人，喜欢养花，交流时偏好温和直接的表达。"}
	svc := NewService(profileStore, generator)
	ctx := context.Background()

	got, err := svc.Update(ctx, UpdateRequest{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:           "蓝",
			Nickname:       "蓝",
			Hobbies:        []string{"养花"},
			AIPortraitMode: PortraitModeAuto,
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(generator.calls) != 1 {
		t.Fatalf("portrait calls = %d, want 1", len(generator.calls))
	}
	if generator.calls[0].Fields.Name != "蓝" || len(generator.calls[0].Fields.Hobbies) != 1 {
		t.Fatalf("portrait input = %+v, want current profile fields", generator.calls[0])
	}
	if got.Fields.AIPortrait != generator.result || got.AIPortraitUpdatedAt == nil {
		t.Fatalf("profile = %+v, want generated portrait and timestamp", got)
	}
	if got.AIPortraitInputHash == "" || !strings.Contains(got.Prompt, "AI认知画像："+generator.result) {
		t.Fatalf("profile = %+v, want portrait fingerprint and device context", got)
	}
}

func TestServiceUpdateManualPortraitNeverCallsGenerator(t *testing.T) {
	_, profileStore := newTestServiceWithStore(t)
	generator := &fakePortraitGenerator{result: "不应写入"}
	svc := NewService(profileStore, generator)

	got, err := svc.Update(context.Background(), UpdateRequest{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:           "蓝",
			AIPortrait:     "管理员确认：性格直爽，喜欢聊足球。",
			AIPortraitMode: PortraitModeManual,
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(generator.calls) != 0 {
		t.Fatalf("portrait calls = %d, want 0 in manual mode", len(generator.calls))
	}
	if got.Fields.AIPortrait != "管理员确认：性格直爽，喜欢聊足球。" {
		t.Fatalf("portrait = %q, want administrator text preserved", got.Fields.AIPortrait)
	}
}

func TestServiceSyncMemoryFactsRefreshesAutoPortraitOnlyWhenInputChanges(t *testing.T) {
	_, profileStore := newTestServiceWithStore(t)
	generator := &fakePortraitGenerator{result: "初始画像"}
	svc := NewService(profileStore, generator)
	ctx := context.Background()

	if _, err := svc.Update(ctx, UpdateRequest{DeviceID: "dev-001", Fields: Fields{Name: "蓝", AIPortraitMode: PortraitModeAuto}}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	generator.result = "喜欢养花，也会关注世界杯。"
	if err := svc.SyncMemoryFacts(ctx, "dev-001", []string{"老人喜欢养花。", "老人关注世界杯。"}); err != nil {
		t.Fatalf("SyncMemoryFacts: %v", err)
	}
	if err := svc.SyncMemoryFacts(ctx, "dev-001", []string{"老人喜欢养花。", "老人关注世界杯。"}); err != nil {
		t.Fatalf("SyncMemoryFacts unchanged: %v", err)
	}
	if len(generator.calls) != 2 {
		t.Fatalf("portrait calls = %d, want initial generation plus one changed-memory refresh", len(generator.calls))
	}
	if got := generator.calls[1].MemoryFacts; len(got) != 2 || got[0] != "老人喜欢养花。" {
		t.Fatalf("memory input = %#v, want current memory facts", got)
	}
	saved, err := svc.Get(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if saved.Fields.AIPortrait != generator.result {
		t.Fatalf("portrait = %q, want refreshed portrait %q", saved.Fields.AIPortrait, generator.result)
	}
}

func TestServiceUpdateMigratesLegacyPortraitOutOfHealth(t *testing.T) {
	svc := newTestService(t)

	got, err := svc.Update(context.Background(), UpdateRequest{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:   "蓝",
			Health: "AI画像：性格开朗，喜欢与家人聊天。\n血压：每日测量",
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Fields.AIPortrait != "性格开朗，喜欢与家人聊天。" || got.Fields.Health != "血压：每日测量" {
		t.Fatalf("fields = %+v, want legacy portrait split from health", got.Fields)
	}
}

func TestBuildStylePromptGuidesFamilyProfileRecall(t *testing.T) {
	prompt := BuildStylePrompt()

	for _, want := range []string{
		"问到家庭成员",
		"依据系统提供的陪伴对象上下文回答",
		"不知道再说明",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want recall guidance %q", prompt, want)
		}
	}
}

func TestBuildStylePromptGuidesCurrentConversationContinuity(t *testing.T) {
	prompt := BuildStylePrompt()

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

func TestBuildStylePromptGuardsDeviceSettingsUnlessElderAsks(t *testing.T) {
	prompt := BuildStylePrompt()

	want := "非老人明确要求，不要更改设备设置、音量、屏幕主题或字体"
	if !strings.Contains(prompt, want) {
		t.Fatalf("prompt = %q, want device settings guard %q", prompt, want)
	}
}

func TestBuildStylePromptUsesConfiguredNicknameVerbatim(t *testing.T) {
	prompt := BuildStylePrompt()

	for _, want := range []string{
		"优先使用陪伴对象上下文中的常用称呼原文",
		"不要自行添加“阿姨”“奶奶”等后缀",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %q, want nickname rule %q", prompt, want)
		}
	}
}

type fakePortraitGenerator struct {
	result string
	err    error
	calls  []PortraitInput
}

func (f *fakePortraitGenerator) GeneratePortrait(_ context.Context, input PortraitInput) (string, error) {
	f.calls = append(f.calls, input)
	return f.result, f.err
}
