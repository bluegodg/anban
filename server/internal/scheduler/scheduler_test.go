package scheduler

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestScheduleAtFires(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	var fired atomic.Int32
	_, err := s.ScheduleAt(time.Now().Add(50*time.Millisecond), func() { fired.Add(1) })
	if err != nil {
		t.Fatalf("ScheduleAt: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	if fired.Load() != 1 {
		t.Fatalf("fired = %d, want 1", fired.Load())
	}
}

func TestCancelStopsOneShot(t *testing.T) {
	s := New()
	s.Start()
	defer s.Stop()

	var fired atomic.Int32
	id, _ := s.ScheduleAt(time.Now().Add(100*time.Millisecond), func() { fired.Add(1) })
	s.Cancel(id)
	time.Sleep(200 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatalf("fired = %d, want 0 (cancelled)", fired.Load())
	}
}

func TestCancelRemovesCronJob(t *testing.T) {
	s := New()

	id, err := s.RegisterCron("* * * * *", func() {})
	if err != nil {
		t.Fatalf("RegisterCron: %v", err)
	}
	if id != "cron-1" {
		t.Fatalf("id = %q, want cron-1", id)
	}

	s.Cancel(id)
	if entry := s.cron.Entry(cron.EntryID(1)); entry.ID != 0 {
		t.Fatalf("cron entry still exists after cancel: %+v", entry)
	}
}
