package timeline

import (
	"errors"
	"time"
)

var ErrInvalidInput = errors.New("timeline: invalid input")

type ItemType string

const (
	ItemChildMessage   ItemType = "child_message"
	ItemElderSpeech    ItemType = "elder_speech"
	ItemAssistantReply ItemType = "assistant_reply"
)

type SourceKind string

const (
	SourceAdmin     SourceKind = "admin"
	SourceMember    SourceKind = "member"
	SourceFamily    SourceKind = "family"
	SourceElder     SourceKind = "elder"
	SourceAssistant SourceKind = "assistant"
)

type Item struct {
	ID          string     `json:"id"`
	Type        ItemType   `json:"type"`
	SourceKind  SourceKind `json:"sourceKind"`
	SourceLabel string     `json:"sourceLabel"`
	Text        string     `json:"text"`
	At          time.Time  `json:"at"`
	Status      string     `json:"status,omitempty"`
	AvatarColor string     `json:"avatarColor,omitempty"`
}

type Response struct {
	DeviceID string `json:"deviceId"`
	Items    []Item `json:"items"`
}

type Request struct {
	DeviceID         string
	ElderDisplayName string
	Limit            int
	Before           *time.Time
}
