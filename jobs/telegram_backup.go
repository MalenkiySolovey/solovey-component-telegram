//go:build !minimal

package jobs

import (
	"context"
	"errors"
	"strings"
	"sync"

	telegramschedule "github.com/MalenkiySolovey/solovey-component-telegram/internal/schedule"
	"github.com/robfig/cron/v3"
)

type TelegramBackupJob struct{ RunBackup func(context.Context) }

func NewTelegramBackupJob(run func(context.Context)) *TelegramBackupJob {
	return &TelegramBackupJob{RunBackup: run}
}

func (j *TelegramBackupJob) Run() {
	j.RunContext(context.Background())
}

func (j *TelegramBackupJob) RunContext(ctx context.Context) {
	if j.RunBackup != nil {
		j.RunBackup(ctx)
	}
}

type TelegramBackupScheduler struct {
	Settings  BackupScheduleSettings
	RunBackup func(context.Context)
	Warning   func(...any)

	cron        entryScheduler
	mu          sync.Mutex
	currentSpec string
	entryID     cron.EntryID
}

type BackupScheduleSettings interface {
	GetTelegramEnabled() (bool, error)
	GetTelegramBackupEnabled() (bool, error)
	GetTelegramBackupCron() (string, error)
}

func NewTelegramBackupScheduler(c entryScheduler, settings BackupScheduleSettings, run func(context.Context), warning func(...any)) *TelegramBackupScheduler {
	return &TelegramBackupScheduler{cron: c, Settings: settings, RunBackup: run, Warning: warning}
}

func (s *TelegramBackupScheduler) Run() {
	s.RunContext(context.Background())
}

func (s *TelegramBackupScheduler) RunContext(ctx context.Context) {
	telegramEnabled, err := s.Settings.GetTelegramEnabled()
	if err != nil {
		s.stopAfterSettingsError(ctx, "telegram enabled", err)
		return
	}
	backupEnabled, err := s.Settings.GetTelegramBackupEnabled()
	if err != nil {
		s.stopAfterSettingsError(ctx, "backup enabled", err)
		return
	}
	spec, err := s.Settings.GetTelegramBackupCron()
	if err != nil {
		s.stopAfterSettingsError(ctx, "backup cron", err)
		return
	}
	spec = strings.TrimSpace(spec)
	if !telegramEnabled || !backupEnabled {
		spec = ""
	}
	if err := s.reconcile(ctx, spec); err != nil {
		s.warn("telegram backup scheduler failed:", err)
	}
}

func (s *TelegramBackupScheduler) stopAfterSettingsError(ctx context.Context, setting string, err error) {
	s.warn("telegram ", setting, " setting read failed:", err)
	if stopErr := s.reconcile(ctx, ""); stopErr != nil {
		s.warn("telegram backup scheduler fail-closed stop failed:", stopErr)
	}
}

func (s *TelegramBackupScheduler) Stop() {
	if err := s.StopContext(context.Background()); err != nil {
		s.warn("telegram backup scheduler stop failed:", err)
	}
}

func (s *TelegramBackupScheduler) StopContext(ctx context.Context) error {
	return s.reconcile(ctx, "")
}

func (s *TelegramBackupScheduler) reconcile(ctx context.Context, spec string) error {
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
	entryID := s.cron.Schedule(schedule, NewTelegramBackupJob(s.RunBackup))
	if entryID == 0 {
		return errors.New("telegram backup scheduler is unavailable")
	}
	s.entryID = entryID
	s.currentSpec = spec
	return nil
}

func (s *TelegramBackupScheduler) warn(args ...any) {
	if s.Warning != nil {
		s.Warning(args...)
	}
}
