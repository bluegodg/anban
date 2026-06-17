package drives

import (
	"reflect"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestActivateReminderDueRaisesStewardshipAndCare(t *testing.T) {
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	got := Activate(
		mind.Situation{DeviceID: "dev-001", At: at, OpenLoops: []string{"reminder_due"}},
		mind.SelfState{Concern: 0.4, StewardWeight: 0.15},
		[]mind.Event{{ID: "evt-1", Type: mind.EventReminderDue}},
	)
	if strength(got, mind.DriveStewardship) < 0.7 {
		t.Fatalf("drives = %+v, want stewardship >= 0.7", got)
	}
	if strength(got, mind.DriveCare) < 0.45 {
		t.Fatalf("drives = %+v, want care >= 0.45", got)
	}
}

func TestActivateLongSilenceRaisesQuietPresence(t *testing.T) {
	got := Activate(
		mind.Situation{DeviceID: "dev-001", ActivityLevel: "low", Constraints: []string{"prefer_observation"}},
		mind.SelfState{Concern: 0.6, Quietness: 0.8},
		[]mind.Event{{ID: "evt-1", Type: mind.EventLongSilence}},
	)
	if strength(got, mind.DriveQuietPresence) < 0.7 {
		t.Fatalf("drives = %+v, want quiet presence high", got)
	}
}

func TestActivateDeduplicatesSourceEventIDsPerDrive(t *testing.T) {
	got := Activate(
		mind.Situation{DeviceID: "dev-001"},
		mind.SelfState{},
		[]mind.Event{
			{ID: "evt-1", Type: mind.EventReminderDue},
			{ID: "evt-2", Type: mind.EventReminderDue},
			{ID: "evt-1", Type: mind.EventReminderDue},
		},
	)

	want := []string{"evt-1", "evt-2"}
	if sources := sourceEventIDs(got, mind.DriveStewardship); !reflect.DeepEqual(sources, want) {
		t.Fatalf("stewardship SourceEventIDs = %+v, want %+v", sources, want)
	}
	if sources := sourceEventIDs(got, mind.DriveCare); !reflect.DeepEqual(sources, want) {
		t.Fatalf("care SourceEventIDs = %+v, want %+v", sources, want)
	}
}

func strength(drives []mind.Drive, name string) float64 {
	for _, drive := range drives {
		if drive.Name == name {
			return drive.Strength
		}
	}
	return 0
}

func sourceEventIDs(drives []mind.Drive, name string) []string {
	for _, drive := range drives {
		if drive.Name == name {
			return drive.SourceEventIDs
		}
	}
	return nil
}
