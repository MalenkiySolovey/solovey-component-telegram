//go:build !minimal

package telegram

import (
	"context"
	"errors"
	"github.com/MalenkiySolovey/solovey-component-telegram/jobs"
	host "github.com/MalenkiySolovey/solovey-host/componenthost"
	"github.com/robfig/cron/v3"
	"sync"
)

// ScheduledJobs owns only Telegram's entries in the injected common scheduler.
// ComponentHost remains the supervisor and admits Start/Stop through its owner.
type ScheduledJobs struct {
	mu              sync.Mutex
	scheduler       host.Scheduler
	entryIDs        []cron.EntryID
	reportScheduler *jobs.TelegramReportScheduler
	backupScheduler *jobs.TelegramBackupScheduler
}

func StartScheduledJobs(scheduler host.Scheduler, cpu cron.Job, report *jobs.TelegramReportScheduler, backup *jobs.TelegramBackupScheduler) (*ScheduledJobs, error) {
	if scheduler == nil || cpu == nil || report == nil || backup == nil {
		return nil, errors.New("telegram scheduled job ports are unavailable")
	}
	active := &ScheduledJobs{scheduler: scheduler, entryIDs: make([]cron.EntryID, 0, 3), reportScheduler: report, backupScheduler: backup}
	report.Run()
	backup.Run()
	for _, job := range []struct {
		spec string
		job  cron.Job
	}{{"@every 12s", cpu}, {"@every 1m", report}, {"@every 1m", backup}} {
		id, err := scheduler.AddJob(job.spec, job.job)
		if err != nil {
			_ = active.Stop(context.Background())
			return nil, err
		}
		active.entryIDs = append(active.entryIDs, id)
	}
	return active, nil
}

func (j *ScheduledJobs) Stop(ctx context.Context) error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.scheduler != nil {
		for i := len(j.entryIDs) - 1; i >= 0; i-- {
			if err := j.scheduler.RemoveJobAndWait(ctx, j.entryIDs[i]); err != nil {
				return err
			}
		}
	}
	if j.reportScheduler != nil {
		if err := j.reportScheduler.StopContext(ctx); err != nil {
			return err
		}
	}
	if j.backupScheduler != nil {
		if err := j.backupScheduler.StopContext(ctx); err != nil {
			return err
		}
	}
	j.scheduler = nil
	j.entryIDs = nil
	j.reportScheduler = nil
	j.backupScheduler = nil
	return nil
}
