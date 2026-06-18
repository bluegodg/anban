package timeline

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

type Service struct {
	messages   sharedtypes.TimelineMessageReader
	xc         xiaozhiclient.Client
	elderNames sharedtypes.ElderDisplayNameReader
}

func NewService(messages sharedtypes.TimelineMessageReader, xc xiaozhiclient.Client, elderNames ...sharedtypes.ElderDisplayNameReader) *Service {
	service := &Service{messages: messages, xc: xc}
	if len(elderNames) > 0 {
		service.elderNames = elderNames[0]
	}
	return service
}

func (s *Service) Get(ctx context.Context, req Request) (Response, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return Response{}, ErrInvalidInput
	}
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	elderName := strings.TrimSpace(req.ElderDisplayName)
	if s.elderNames != nil {
		if profileName, err := s.elderNames.GetElderDisplayName(ctx, deviceID); err == nil && strings.TrimSpace(profileName) != "" {
			elderName = strings.TrimSpace(profileName)
		}
	}
	if elderName == "" {
		elderName = "老人"
	}

	items := make([]Item, 0, limit)
	if s.messages != nil {
		messages, err := s.messages.ListTimelineMessages(ctx, deviceID, limit)
		if err != nil {
			return Response{}, err
		}
		for _, msg := range messages {
			at := msg.QueuedAt.UTC()
			if req.Before != nil && !at.Before(req.Before.UTC()) {
				continue
			}
			sourceKind := SourceFamily
			switch strings.TrimSpace(msg.SenderRole) {
			case "admin":
				sourceKind = SourceAdmin
			case "member":
				sourceKind = SourceMember
			}
			items = append(items, Item{
				ID:          fmt.Sprintf("msg-%d", msg.MessageID),
				Type:        ItemChildMessage,
				SourceKind:  sourceKind,
				SourceLabel: msg.SenderDisplayName,
				Text:        msg.Text,
				At:          at,
				Status:      msg.Status,
				AvatarColor: msg.AvatarColor,
			})
		}
	}

	if s.xc != nil {
		history, err := s.xc.GetHistory(ctx, deviceID, limit)
		if err == nil {
			for i, entry := range history {
				at := entry.At.UTC()
				if at.IsZero() || (req.Before != nil && !at.Before(req.Before.UTC())) {
					continue
				}
				role := strings.ToLower(strings.TrimSpace(entry.Role))
				text := strings.TrimSpace(entry.Text)
				if text == "" {
					continue
				}
				item := Item{
					ID:   fmt.Sprintf("hist-%s-%d-%d", role, at.UnixNano(), i),
					Text: text,
					At:   at,
				}
				switch role {
				case "user":
					item.Type = ItemElderSpeech
					item.SourceKind = SourceElder
					item.SourceLabel = elderName
				case "assistant":
					item.Type = ItemAssistantReply
					item.SourceKind = SourceAssistant
					item.SourceLabel = "安伴"
				default:
					continue
				}
				items = append(items, item)
			}
		} else if !errors.Is(err, sharedtypes.ErrNotImplemented) {
			// History is an enrichment. Messages remain useful when xiaozhi is temporarily unavailable.
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].At.Equal(items[j].At) {
			return items[i].ID < items[j].ID
		}
		return items[i].At.Before(items[j].At)
	})
	if len(items) > limit {
		items = items[len(items)-limit:]
	}
	return Response{DeviceID: deviceID, Items: items}, nil
}
