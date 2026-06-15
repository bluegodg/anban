package profile

import (
	"context"
	"errors"
	"strings"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type Service struct {
	store *Store
	xc    xiaozhiclient.Client
}

const (
	maxProfilePromptRunes     = 1500
	maxProfilePromptLineRunes = 160
)

func NewService(store *Store, xc xiaozhiclient.Client) *Service {
	return &Service{store: store, xc: xc}
}

func (s *Service) Update(ctx context.Context, req UpdateRequest) (Profile, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return Profile{}, ErrInvalidInput
	}

	fields := normalizeFields(req.Fields)
	prompt := BuildPrompt(fields)
	profile := Profile{
		DeviceID: deviceID,
		Fields:   fields,
		Prompt:   prompt,
	}
	if err := s.store.Upsert(ctx, &profile); err != nil {
		return Profile{}, err
	}
	if err := s.xc.SetRolePrompt(ctx, deviceID, prompt); err != nil {
		return profile, err
	}
	return profile, nil
}

func (s *Service) Get(ctx context.Context, deviceID string) (Profile, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return Profile{}, ErrInvalidInput
	}
	return s.store.Get(ctx, deviceID)
}

func BuildPrompt(fields Fields) string {
	return BuildPromptWithMemory(fields, nil)
}

func BuildPromptWithMemory(fields Fields, memoryFacts []string) string {
	lines := []string{
		"你是安伴，一位温和、耐心、像家人一样陪伴老人的语音助手。",
		"请优先使用下面的家庭画像理解老人，不要生硬复述画像内容，回答要自然、简短、关心当下。",
		"老人问到子女或孙辈姓名、称呼、喜好、健康或忌口时，直接依据家庭画像回答名字或事实；不知道再说明。",
		"当前会话中老人刚说过的事也要当作短期上下文，后续回答要自然承接，不要像第一次听到一样重复追问。",
		"非老人明确要求，不要更改设备设置/音量/屏幕主题/字体；日常陪伴中不要主动调用设备设置工具。",
	}
	addLine := func(label, value string) {
		if value != "" {
			lines = appendPromptLine(lines, label+"："+value)
		}
	}

	addLine("老人本名", fields.Name)
	addLine("常用称呼", fields.Nickname)
	addLine("子女", strings.Join(fields.Children, "、"))
	addLine("孙辈", strings.Join(fields.Grandchildren, "、"))
	addLine("喜好", strings.Join(fields.Hobbies, "、"))
	addLine("作息", fields.Schedule)
	addLine("健康背景", fields.Health)
	addLine("忌口和禁忌", strings.Join(fields.Taboos, "、"))
	if facts := trimStrings(memoryFacts); len(facts) > 0 {
		appendMemoryFact := func(value string) {
			lines = appendPromptLine(lines, "近期记忆："+value)
		}
		for _, fact := range facts {
			appendMemoryFact(fact)
		}
	}
	return strings.Join(lines, "\n")
}

func (s *Service) SyncMemoryFacts(ctx context.Context, deviceID string, facts []string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ErrInvalidInput
	}

	current, err := s.store.Get(ctx, deviceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if errors.Is(err, ErrNotFound) {
		current = Profile{DeviceID: deviceID}
	}
	current.Prompt = BuildPromptWithMemory(current.Fields, facts)
	if err := s.store.Upsert(ctx, &current); err != nil {
		return err
	}
	return s.xc.SetRolePrompt(ctx, deviceID, current.Prompt)
}

func appendPromptLine(lines []string, line string) []string {
	line = truncateRunes(line, maxProfilePromptLineRunes)
	available := maxProfilePromptRunes - promptRuneLen(lines)
	if len(lines) > 0 {
		available--
	}
	if available <= 0 {
		return lines
	}
	line = truncateRunes(line, available)
	if line == "" {
		return lines
	}
	return append(lines, line)
}

func promptRuneLen(lines []string) int {
	total := 0
	for i, line := range lines {
		if i > 0 {
			total++
		}
		total += len([]rune(line))
	}
	return total
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func normalizeFields(fields Fields) Fields {
	fields.Name = strings.TrimSpace(fields.Name)
	fields.Nickname = strings.TrimSpace(fields.Nickname)
	fields.Children = trimStrings(fields.Children)
	fields.Grandchildren = trimStrings(fields.Grandchildren)
	fields.Hobbies = trimStrings(fields.Hobbies)
	fields.Schedule = strings.TrimSpace(fields.Schedule)
	fields.Health = strings.TrimSpace(fields.Health)
	fields.Taboos = trimStrings(fields.Taboos)
	return fields
}

func trimStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
