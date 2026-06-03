package proactive

import (
	"context"
	"errors"
	"testing"
	"time"

	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

func TestVoiceGateLimitsDeviceToOneProactiveVoicePerWindow(t *testing.T) {
	gate := NewVoiceGate(10 * time.Minute)
	ctx := context.Background()
	at := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)

	lease, err := gate.TryAcquireProactiveVoice(ctx, "dev-001", at)
	if err != nil {
		t.Fatalf("first TryAcquireProactiveVoice: %v", err)
	}
	if err := lease.Commit(ctx); err != nil {
		t.Fatalf("Commit first lease: %v", err)
	}

	if _, err := gate.TryAcquireProactiveVoice(ctx, "dev-001", at.Add(9*time.Minute+59*time.Second)); !errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
		t.Fatalf("second TryAcquireProactiveVoice err = %v, want ErrProactiveVoiceThrottled", err)
	}

	if _, err := gate.TryAcquireProactiveVoice(ctx, "dev-001", at.Add(10*time.Minute)); err != nil {
		t.Fatalf("TryAcquireProactiveVoice after window: %v", err)
	}
}

func TestVoiceGateValidatesInputAndHonorsCanceledContext(t *testing.T) {
	gate := NewVoiceGate(10 * time.Minute)
	ctx := context.Background()
	at := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)

	if _, err := gate.TryAcquireProactiveVoice(ctx, " ", at); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank device err = %v, want ErrInvalidInput", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := gate.TryAcquireProactiveVoice(canceled, "dev-001", at); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled TryAcquireProactiveVoice err = %v, want context.Canceled", err)
	}

	lease, err := gate.TryAcquireProactiveVoice(ctx, "dev-001", at)
	if err != nil {
		t.Fatalf("TryAcquireProactiveVoice: %v", err)
	}
	if err := lease.Commit(canceled); err != nil {
		t.Fatalf("Commit with canceled context: %v", err)
	}
	if _, err := gate.TryAcquireProactiveVoice(ctx, "dev-001", at.Add(time.Minute)); !errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
		t.Fatalf("TryAcquireProactiveVoice after canceled Commit err = %v, want ErrProactiveVoiceThrottled", err)
	}

	rollbackLease, err := gate.TryAcquireProactiveVoice(ctx, "dev-002", at)
	if err != nil {
		t.Fatalf("TryAcquireProactiveVoice for rollback lease: %v", err)
	}
	if err := rollbackLease.Rollback(canceled); err != nil {
		t.Fatalf("Rollback with canceled context: %v", err)
	}
	if _, err := gate.TryAcquireProactiveVoice(ctx, "dev-002", at.Add(time.Minute)); err != nil {
		t.Fatalf("TryAcquireProactiveVoice after canceled Rollback: %v", err)
	}
}

func TestVoiceGateIsPerDeviceAndRollsBackFailedAttempts(t *testing.T) {
	gate := NewVoiceGate(10 * time.Minute)
	ctx := context.Background()
	at := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)

	lease, err := gate.TryAcquireProactiveVoice(ctx, "dev-001", at)
	if err != nil {
		t.Fatalf("TryAcquireProactiveVoice: %v", err)
	}
	if err := lease.Rollback(ctx); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if _, err := gate.TryAcquireProactiveVoice(ctx, "dev-001", at.Add(time.Minute)); err != nil {
		t.Fatalf("TryAcquireProactiveVoice after rollback: %v", err)
	}

	otherLease, err := gate.TryAcquireProactiveVoice(ctx, "dev-002", at)
	if err != nil {
		t.Fatalf("TryAcquireProactiveVoice for second device: %v", err)
	}
	if err := otherLease.Commit(ctx); err != nil {
		t.Fatalf("Commit second device lease: %v", err)
	}
}
