package proactive

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

var ErrInvalidInput = errors.New("proactive: invalid input")

type VoiceGate struct {
	window       time.Duration
	mu           sync.Mutex
	lastPlayed   map[string]time.Time
	reservations map[string]time.Time
}

func NewVoiceGate(window time.Duration) *VoiceGate {
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &VoiceGate{
		window:       window,
		lastPlayed:   make(map[string]time.Time),
		reservations: make(map[string]time.Time),
	}
}

func (g *VoiceGate) TryAcquireProactiveVoice(ctx context.Context, deviceID string, at time.Time) (sharedtypes.ProactiveVoiceLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, ErrInvalidInput
	}
	if at.IsZero() {
		at = time.Now()
	}
	at = at.UTC()

	g.mu.Lock()
	defer g.mu.Unlock()

	if last, ok := g.lastPlayed[deviceID]; ok && at.Sub(last) < g.window {
		return nil, sharedtypes.ErrProactiveVoiceThrottled
	}
	if reserved, ok := g.reservations[deviceID]; ok && at.Sub(reserved) < g.window {
		return nil, sharedtypes.ErrProactiveVoiceThrottled
	}

	g.reservations[deviceID] = at
	return &voiceLease{gate: g, deviceID: deviceID, at: at}, nil
}

type voiceLease struct {
	gate     *VoiceGate
	deviceID string
	at       time.Time
	done     bool
}

func (l *voiceLease) Commit(ctx context.Context) error {
	if l.done {
		return nil
	}

	l.gate.mu.Lock()
	defer l.gate.mu.Unlock()

	if reserved, ok := l.gate.reservations[l.deviceID]; ok && reserved.Equal(l.at) {
		delete(l.gate.reservations, l.deviceID)
	}
	l.gate.lastPlayed[l.deviceID] = l.at
	l.done = true
	return nil
}

func (l *voiceLease) Rollback(ctx context.Context) error {
	if l.done {
		return nil
	}

	l.gate.mu.Lock()
	defer l.gate.mu.Unlock()

	if reserved, ok := l.gate.reservations[l.deviceID]; ok && reserved.Equal(l.at) {
		delete(l.gate.reservations, l.deviceID)
	}
	l.done = true
	return nil
}
