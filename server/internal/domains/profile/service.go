package profile

import (
	"context"
	"strings"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type Service struct {
	store *Store
	xc    xiaozhiclient.Client
}

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
	lines := []string{
		"你是安伴，一位温和、耐心、像家人一样陪伴老人的语音助手。",
		"请优先使用下面的家庭画像理解老人，不要生硬复述画像内容，回答要自然、简短、关心当下。",
	}
	addLine := func(label, value string) {
		if value != "" {
			lines = append(lines, label+"："+value)
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
	return strings.Join(lines, "\n")
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
