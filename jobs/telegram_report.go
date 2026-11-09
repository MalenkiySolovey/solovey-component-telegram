//go:build !minimal

package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	telegramschedule "github.com/MalenkiySolovey/solovey-component-telegram/internal/schedule"
	"github.com/robfig/cron/v3"
)

type TelegramReportJob struct {
	Notify func(string, map[string]string)
}

func NewTelegramReportJob(notify func(string, map[string]string)) *TelegramReportJob {
	return &TelegramReportJob{Notify: notify}
}

func (j *TelegramReportJob) Run() {
	j.RunContext(context.Background())
}

func (j *TelegramReportJob) RunContext(ctx context.Context) {
	if ctx == nil || ctx.Err() != nil {
		return
	}
	j.Notify("scheduled_report", map[string]string{
		"ts": time.Now().UTC().Format(time.RFC3339),
	})
}

type TelegramReportScheduler struct {
	Settings ReportSettings
	Notify   func(string, map[string]string)
	Warning  func(...any)

	cron        entryScheduler
	mu          sync.Mutex
	currentSpec string
	entryID     cron.EntryID
}

type entryScheduler interface {
	Schedule(cron.Schedule, cron.Job) cron.EntryID
	RemoveJobAndWait(context.Context, cron.EntryID) error
}

type ReportSettings interface {
	GetTelegramEnabled() (bool, error)
	GetTelegramReport() (bool, error)
	GetTelegramReportCron() (string, error)
}

func NewTelegramReportScheduler(c entryScheduler, settings ReportSettings, notify func(string, map[string]string), warning func(...any)) *TelegramReportScheduler {
	return &TelegramReportScheduler{cron: c, Settings: settings, Notify: notify, Warning: warning}
}

func (s *TelegramReportScheduler) Run() {
	s.RunContext(context.Background())
}

func (s *TelegramReportScheduler) RunContext(ctx context.Context) {
	telegramEnabled, err := s.Settings.GetTelegramEnabled()
	if err != nil {
		s.stopAfterSettingsError(ctx, "telegram enabled", err)
		return
	}
	enabled, err := s.Settings.GetTelegramReport()
	if err != nil {
		s.stopAfterSettingsError(ctx, "report enabled", err)
		return
	}
	spec, err := s.Settings.GetTelegramReportCron()
	if err != nil {
		s.stopAfterSettingsError(ctx, "report cron", err)
		return
	}
	if !telegramEnabled || !enabled {
		spec = ""
	}
	if err := s.reconcile(ctx, spec); err != nil {
		s.warn("telegram report scheduler failed:", err)
	}
}

func (s *TelegramReportScheduler) stopAfterSettingsError(ctx context.Context, setting string, err error) {
	s.warn("telegram ", setting, " setting read failed:", err)
	if stopErr := s.reconcile(ctx, ""); stopErr != nil {
		s.warn("telegram report scheduler fail-closed stop failed:", stopErr)
	}
}

func (s *TelegramReportScheduler) Stop() {
	if err := s.StopContext(context.Background()); err != nil {
		s.warn("telegram report scheduler stop failed:", err)
	}
}

func (s *TelegramReportScheduler) StopContext(ctx context.Context) error {
	return s.reconcile(ctx, "")
}

func (s *TelegramReportScheduler) reconcile(ctx context.Context, spec string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if spec == s.currentSpec {
		return nil
	}
	if s.entryID != 0 {
		if err := s.cron.RemoveJobAndWait(ctx, s.entryID); err != nil {
			return err
		}
		s.entryID = 0
	}
	s.currentSpec = ""
	if spec == "" {
		return nil
	}
	schedule, err := telegramschedule.Parse(spec)
	if err != nil {
		return err
	}
	entryID := s.cron.Schedule(schedule, NewTelegramReportJob(s.Notify))
	if entryID == 0 {
		return errors.New("telegram report scheduler is unavailable")
	}
	s.entryID = entryID
	s.currentSpec = spec
	return nil
}

func (s *TelegramReportScheduler) warn(args ...any) {
	if s.Warning != nil {
		s.Warning(args...)
	}
}
