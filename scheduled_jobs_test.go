//go:build !minimal

package telegram

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/MalenkiySolovey/solovey-component-telegram/jobs"
	"github.com/robfig/cron/v3"
)

type disabledScheduleSettings struct{}

func (disabledScheduleSettings) GetTelegramEnabled() (bool, error)       { return false, nil }
func (disabledScheduleSettings) GetTelegramReport() (bool, error)        { return false, nil }
func (disabledScheduleSettings) GetTelegramReportCron() (string, error)  { return "", nil }
func (disabledScheduleSettings) GetTelegramBackupEnabled() (bool, error) { return false, nil }
func (disabledScheduleSettings) GetTelegramBackupCron() (string, error)  { return "", nil }

type schedulerFixture struct {
	added   int
	failAt  int
	removed []cron.EntryID
	stopErr error
}

func (s *schedulerFixture) AddJob(_ string, _ cron.Job) (cron.EntryID, error) {
	s.added++
	if s.added == s.failAt {
		return 0, errors.New("registration failed")
	}
	return cron.EntryID(s.added), nil
}
func (*schedulerFixture) Schedule(cron.Schedule, cron.Job) cron.EntryID { return 0 }
func (*schedulerFixture) RemoveJob(cron.EntryID)                        {}
func (s *schedulerFixture) RemoveJobAndWait(_ context.Context, id cron.EntryID) error {
	if s.stopErr != nil {
		return s.stopErr
	}
	s.removed = append(s.removed, id)
	return nil
}

func scheduledFixture(s *schedulerFixture) (*jobs.TelegramReportScheduler, *jobs.TelegramBackupScheduler) {
	return jobs.NewTelegramReportScheduler(s, disabledScheduleSettings{}, func(string, map[string]string) {}, nil), jobs.NewTelegramBackupScheduler(s, disabledScheduleSettings{}, func(context.Context) {}, nil)
}
func TestImportAndConstructionDoNotStartFeatureWork(t *testing.T) {
	s := &schedulerFixture{}
	report, backup := scheduledFixture(s)
	if s.added != 0 || report == nil || backup == nil {
		t.Fatal("construction registered recurring work")
	}
	if Manifest().ID != "telegram" {
		t.Fatal("manifest identity changed")
	}
}
func TestScheduledJobsCompensatePartialStartAndDrainIdempotently(t *testing.T) {
	for _, failAt := range []int{0, 2, 3} {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			s := &schedulerFixture{failAt: failAt}
			report, backup := scheduledFixture(s)
			active, err := StartScheduledJobs(s, cron.FuncJob(func() {}), report, backup)
			if failAt != 0 {
				if err == nil || active != nil {
					t.Fatal("failed start accepted")
				}
				expected := []cron.EntryID{1}
				if failAt == 3 {
					expected = []cron.EntryID{2, 1}
				}
				if !reflect.DeepEqual(s.removed, expected) {
					t.Fatalf("partial compensation=%v", s.removed)
				}
				return
			}
			if err != nil || s.added != 3 {
				t.Fatalf("start=%v registrations=%d", err, s.added)
			}
			s.stopErr = context.DeadlineExceeded
			if err := active.Stop(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("deadline drain reported success")
			}
			s.stopErr = nil
			if err := active.Stop(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := active.Stop(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.removed, []cron.EntryID{3, 2, 1}) {
				t.Fatalf("drain not idempotent: %v", s.removed)
			}
		})
	}
}
