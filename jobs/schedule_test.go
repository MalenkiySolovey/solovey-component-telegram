//go:build !minimal

package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/robfig/cron/v3"
)

type scheduleSettings struct {
	enabled, report, backup bool
	reportCron, backupCron  string
	err                     error
}

func (s *scheduleSettings) GetTelegramEnabled() (bool, error)       { return s.enabled, s.err }
func (s *scheduleSettings) GetTelegramReport() (bool, error)        { return s.report, nil }
func (s *scheduleSettings) GetTelegramReportCron() (string, error)  { return s.reportCron, nil }
func (s *scheduleSettings) GetTelegramBackupEnabled() (bool, error) { return s.backup, nil }
func (s *scheduleSettings) GetTelegramBackupCron() (string, error)  { return s.backupCron, nil }

type entryFixture struct {
	*cron.Cron
	stopErr error
	reject  bool
}

func (s *entryFixture) Schedule(spec cron.Schedule, job cron.Job) cron.EntryID {
	if s.reject {
		return 0
	}
	return s.Cron.Schedule(spec, job)
}
func (s *entryFixture) RemoveJobAndWait(_ context.Context, id cron.EntryID) error {
	if s.stopErr != nil {
		return s.stopErr
	}
	s.Remove(id)
	return nil
}

func TestReportScheduleReplansAndFailsClosedOnSettingsError(t *testing.T) {
	settings := &scheduleSettings{enabled: true, report: true, reportCron: "*/5 * * * *"}
	cron := &entryFixture{Cron: cron.New()}
	scheduler := NewTelegramReportScheduler(cron, settings, func(string, map[string]string) {}, nil)
	scheduler.Run()
	first := scheduler.entryID
	if first == 0 || scheduler.currentSpec != "*/5 * * * *" {
		t.Fatal("report not scheduled")
	}
	settings.reportCron = "*/10 * * * *"
	scheduler.Run()
	if scheduler.entryID == 0 || scheduler.entryID == first || len(cron.Entries()) != 1 {
		t.Fatal("report not replanned exactly once")
	}
	settings.err = errors.New("unavailable")
	scheduler.Run()
	if scheduler.entryID != 0 || scheduler.currentSpec != "" || len(cron.Entries()) != 0 {
		t.Fatal("report retained after settings failure")
	}
}

func TestBackupScheduleDrainFailureRetainsStateForRetry(t *testing.T) {
	settings := &scheduleSettings{enabled: true, backup: true, backupCron: "*/5 * * * *"}
	cron := &entryFixture{Cron: cron.New()}
	scheduler := NewTelegramBackupScheduler(cron, settings, func(context.Context) {}, nil)
	scheduler.Run()
	first := scheduler.entryID
	cron.stopErr = context.DeadlineExceeded
	if err := scheduler.StopContext(t.Context()); !errors.Is(err, context.DeadlineExceeded) || scheduler.entryID != first || first == 0 {
		t.Fatal("failed drain lost active entry")
	}
	cron.stopErr = nil
	if err := scheduler.StopContext(t.Context()); err != nil || scheduler.entryID != 0 || len(cron.Entries()) != 0 {
		t.Fatal("drain retry failed")
	}
	scheduler.Run()
	settings.enabled = false
	scheduler.Run()
	if scheduler.entryID != 0 || len(cron.Entries()) != 0 {
		t.Fatal("disabled backup retained job")
	}
}

func TestSchedulersRejectInvalidAndUnavailableSchedule(t *testing.T) {
	settings := &scheduleSettings{enabled: true, report: true, backup: true, reportCron: "invalid", backupCron: "invalid"}
	cron := &entryFixture{Cron: cron.New()}
	report := NewTelegramReportScheduler(cron, settings, func(string, map[string]string) {}, nil)
	backup := NewTelegramBackupScheduler(cron, settings, func(context.Context) {}, nil)
	report.Run()
	backup.Run()
	if report.entryID != 0 || backup.entryID != 0 || len(cron.Entries()) != 0 {
		t.Fatal("invalid schedule admitted")
	}
	settings.reportCron = "*/5 * * * *"
	settings.backupCron = settings.reportCron
	cron.reject = true
	report.Run()
	backup.Run()
	if report.entryID != 0 || backup.entryID != 0 || report.currentSpec != "" || backup.currentSpec != "" {
		t.Fatal("failed registration retained state")
	}
}

func TestReportAndBackupJobsPublishOnlyThroughInjectedOwner(t *testing.T) {
	var event string
	report := NewTelegramReportJob(func(name string, fields map[string]string) {
		event = name
		if fields["ts"] == "" {
			t.Fatal("missing report timestamp")
		}
	})
	report.RunContext(t.Context())
	if event != "scheduled_report" {
		t.Fatal("report event changed")
	}
	calls := 0
	backup := NewTelegramBackupJob(func(context.Context) { calls++ })
	backup.RunContext(t.Context())
	if calls != 1 {
		t.Fatal("backup owner not invoked exactly once")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	event = ""
	report.RunContext(canceled)
	if event != "" {
		t.Fatal("canceled report was published")
	}
}
