package profile

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type Service struct {
	store     *Store
	generator PortraitGenerator
}

const (
	maxProfilePromptRunes     = 1500
	maxProfilePromptLineRunes = 160
	maxMindContextLineRunes   = 360
	portraitFingerprintV2     = "ai-portrait-v2"
)

func NewService(store *Store, generators ...PortraitGenerator) *Service {
	service := &Service{store: store}
	if len(generators) > 0 {
		service.generator = generators[0]
	}
	return service
}

func (s *Service) Update(ctx context.Context, req UpdateRequest) (Profile, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return Profile{}, ErrInvalidInput
	}

	requestedMode := strings.TrimSpace(req.Fields.AIPortraitMode)
	fields := normalizeFields(req.Fields)
	current, err := s.store.Get(ctx, deviceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Profile{}, err
	}
	if errors.Is(err, ErrNotFound) {
		current = Profile{DeviceID: deviceID}
	}
	previousFields := current.Fields
	fields = resolvePortraitFields(fields, previousFields, requestedMode != "")
	current.Fields = fields
	current.MemoryFacts = trimStrings(current.MemoryFacts)
	current.MindContext = strings.TrimSpace(current.MindContext)
	if fields.AIPortraitMode == PortraitModeManual {
		current.AIPortraitInputHash = ""
		if fields.AIPortrait != previousFields.AIPortrait || previousFields.AIPortraitMode != PortraitModeManual {
			now := time.Now().UTC()
			current.AIPortraitUpdatedAt = &now
		}
	} else {
		if previousFields.AIPortraitMode != PortraitModeAuto {
			current.AIPortraitInputHash = ""
		}
		s.refreshAIPortrait(ctx, &current)
	}
	current.Prompt = BuildPromptWith(current.Fields, current.MemoryFacts, current.MindContext)
	if err := s.store.Upsert(ctx, &current); err != nil {
		return Profile{}, err
	}
	return current, nil
}

func (s *Service) Get(ctx context.Context, deviceID string) (Profile, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return Profile{}, ErrInvalidInput
	}
	return s.store.Get(ctx, deviceID)
}

func (s *Service) GetElderDisplayName(ctx context.Context, deviceID string) (string, error) {
	current, err := s.Get(ctx, deviceID)
	if err != nil {
		return "", err
	}
	if nickname := strings.TrimSpace(current.Fields.Nickname); nickname != "" {
		return nickname, nil
	}
	return strings.TrimSpace(current.Fields.Name), nil
}

func BuildPrompt(fields Fields) string {
	return BuildPromptWith(fields, nil, "")
}

func BuildPromptWithMemory(fields Fields, memoryFacts []string) string {
	return BuildPromptWith(fields, memoryFacts, "")
}

func BuildPromptWith(fields Fields, memoryFacts []string, mindContext string) string {
	lines := []string{}
	addLine := func(label, value string) {
		if value != "" {
			lines = appendPromptLine(lines, label+"："+value)
		}
	}

	addLine("陪伴对象姓名", fields.Name)
	addLine("常用称呼", fields.Nickname)
	addLine("子女", strings.Join(fields.Children, "、"))
	addLine("孙辈", strings.Join(fields.Grandchildren, "、"))
	addLine("喜好", strings.Join(fields.Hobbies, "、"))
	addLine("作息", fields.Schedule)
	addLine("AI认知画像", fields.AIPortrait)
	addLine("健康背景", fields.Health)
	addLine("忌口和禁忌", strings.Join(fields.Taboos, "、"))
	if facts := trimStrings(memoryFacts); len(facts) > 0 {
		appendMemoryFact := func(value string) {
			lines = appendPromptLine(lines, "专属记忆："+value)
		}
		for _, fact := range facts {
			appendMemoryFact(fact)
		}
	}
	if mindContext = strings.TrimSpace(mindContext); mindContext != "" {
		lines = appendPromptLineWithLimit(lines, "心智上下文："+mindContext, maxMindContextLineRunes)
	}
	return strings.Join(lines, "\n")
}

// BuildStylePrompt returns the manager-owned style layer. It deliberately
// contains no companion profile, memory, or mind state.
func BuildStylePrompt() string {
	return strings.Join([]string{
		"你是安伴，一位温和、耐心、像家人一样陪伴老人的语音助手。",
		"回答要自然、简短、关心当下，不要生硬复述背景资料。",
		"问到家庭成员、喜好、健康或忌口时，依据系统提供的陪伴对象上下文回答；不知道再说明。",
		"称呼对方时优先使用陪伴对象上下文中的常用称呼原文，不要自行添加“阿姨”“奶奶”等后缀。",
		"当前会话中老人刚说过的事要记住，后续回答要自然承接，不要像第一次听到一样重复追问。",
		"非老人明确要求，不要更改设备设置、音量、屏幕主题或字体；日常陪伴中不要主动调用设备设置工具。",
	}, "\n")
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
	current.Fields = normalizeStoredFields(current.Fields)
	current.MemoryFacts = trimStrings(facts)
	current.MindContext = strings.TrimSpace(current.MindContext)
	s.refreshAIPortrait(ctx, &current)
	current.Prompt = BuildPromptWith(current.Fields, current.MemoryFacts, current.MindContext)
	return s.store.Upsert(ctx, &current)
}

func (s *Service) SyncMindContext(ctx context.Context, deviceID string, mindContext string) error {
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
	current.Fields = normalizeStoredFields(current.Fields)
	current.MemoryFacts = trimStrings(current.MemoryFacts)
	current.MindContext = strings.TrimSpace(mindContext)
	current.Prompt = BuildPromptWith(current.Fields, current.MemoryFacts, current.MindContext)
	return s.store.Upsert(ctx, &current)
}

func (s *Service) RefreshAIPortrait(ctx context.Context, deviceID string) (Profile, error) {
	current, err := s.Get(ctx, deviceID)
	if err != nil {
		return Profile{}, err
	}
	current.Fields = normalizeStoredFields(current.Fields)
	s.refreshAIPortrait(ctx, &current)
	current.Prompt = BuildPromptWith(current.Fields, current.MemoryFacts, current.MindContext)
	if err := s.store.Upsert(ctx, &current); err != nil {
		return Profile{}, err
	}
	return current, nil
}

func (s *Service) refreshAIPortrait(ctx context.Context, current *Profile) {
	if current == nil || s.generator == nil || current.Fields.AIPortraitMode != PortraitModeAuto {
		return
	}
	profileFields := current.Fields
	profileFields.AIPortrait = ""
	profileFields.AIPortraitMode = ""
	profileContext := BuildPromptWith(profileFields, nil, "")
	facts := trimStrings(current.MemoryFacts)
	if profileContext == "" && len(facts) == 0 {
		return
	}
	fingerprint := portraitInputFingerprint(profileContext, facts)
	if current.AIPortraitInputHash == fingerprint && strings.TrimSpace(current.Fields.AIPortrait) != "" {
		return
	}

	portrait, err := s.generator.GeneratePortrait(ctx, PortraitInput{
		Fields:           current.Fields,
		MemoryFacts:      append([]string(nil), facts...),
		PreviousPortrait: current.Fields.AIPortrait,
	})
	if err != nil {
		log.Printf("profile AI portrait 生成失败 device=%s: %v", current.DeviceID, err)
		return
	}
	portrait = truncateRunes(strings.TrimSpace(portrait), 360)
	if portrait == "" {
		return
	}
	current.Fields.AIPortrait = portrait
	current.AIPortraitInputHash = fingerprint
	now := time.Now().UTC()
	current.AIPortraitUpdatedAt = &now
}

func portraitInputFingerprint(profileContext string, facts []string) string {
	sum := sha256.Sum256([]byte(portraitFingerprintV2 + "\n" + profileContext + "\n" + strings.Join(facts, "\n")))
	return fmt.Sprintf("%x", sum[:])
}

func appendPromptLine(lines []string, line string) []string {
	return appendPromptLineWithLimit(lines, line, maxProfilePromptLineRunes)
}

func appendPromptLineWithLimit(lines []string, line string, lineLimit int) []string {
	line = truncateRunes(line, lineLimit)
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
	fields.AIPortrait = strings.TrimSpace(fields.AIPortrait)
	fields.AIPortraitMode = normalizePortraitMode(fields.AIPortraitMode)
	legacyPortrait, health := splitLegacyPortrait(fields.Health)
	if fields.AIPortrait == "" {
		fields.AIPortrait = legacyPortrait
	}
	fields.Health = health
	return fields
}

func normalizeStoredFields(fields Fields) Fields {
	fields = normalizeFields(fields)
	if fields.AIPortraitMode == "" {
		fields.AIPortraitMode = PortraitModeAuto
	}
	return fields
}

func resolvePortraitFields(fields, current Fields, modeExplicit bool) Fields {
	current = normalizeStoredFields(current)
	if fields.AIPortraitMode == "" {
		fields.AIPortraitMode = current.AIPortraitMode
		if fields.AIPortraitMode == "" {
			fields.AIPortraitMode = PortraitModeAuto
		}
	}
	if fields.AIPortraitMode == PortraitModeAuto {
		if current.AIPortrait != "" {
			fields.AIPortrait = current.AIPortrait
		}
	} else if !modeExplicit && fields.AIPortrait == "" {
		fields.AIPortrait = current.AIPortrait
	}
	return fields
}

func normalizePortraitMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case PortraitModeAuto:
		return PortraitModeAuto
	case PortraitModeManual:
		return PortraitModeManual
	default:
		return ""
	}
}

func splitLegacyPortrait(health string) (string, string) {
	var portrait string
	lines := strings.Split(strings.TrimSpace(health), "\n")
	healthLines := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "AI画像：") || strings.HasPrefix(line, "AI认知画像：") {
			if portrait == "" {
				portrait = strings.TrimSpace(strings.SplitN(line, "：", 2)[1])
			}
			continue
		}
		healthLines = append(healthLines, line)
	}
	return portrait, strings.Join(healthLines, "\n")
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
