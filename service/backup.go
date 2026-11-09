//go:build !minimal

package telegram

import (
	"context"
	"github.com/MalenkiySolovey/solovey-host/scheduler"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/MalenkiySolovey/solovey-host/redact"
)

const (
	TelegramBackupTriggerManual    = "manual"
	TelegramBackupTriggerScheduled = "scheduled"
)

type TelegramBackupResult struct {
	Success           bool     `json:"success"`
	Filename          string   `json:"filename,omitempty"`
	Trigger           string   `json:"trigger,omitempty"`
	ErrorClass        string   `json:"errorClass,omitempty"`
	PayloadSizeBytes  int64    `json:"-"`
	EnvelopeSizeBytes int64    `json:"-"`
	ExcludedTables    []string `json:"-"`
}

// PreparedBackup is an encrypted stream accepted by the existing Backup owner.
// Closing Content releases its owner-created temporary artifact.
type PreparedBackup struct {
	Content                             io.ReadCloser
	PayloadSizeBytes, EnvelopeSizeBytes int64
}

type TelegramBackupService struct {
	Operations         scheduler.OperationLease
	Prepare            func(context.Context, string) (PreparedBackup, string)
	ParseExcludes      func(string) []string
	Warning            func(...any)
	Settings           BackupSettings
	Telegram           *Service
	Audit              func(AuditRecord) error
	SendDocumentStream func(context.Context, string, io.Reader, string) Result
}

type telegramBackupActorContextKey struct{}

var telegramBackupRunMu sync.Mutex

func ContextWithTelegramBackupActor(ctx context.Context, actor string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, telegramBackupActorContextKey{}, actor)
}

func (s *TelegramBackupService) RunOnce(ctx context.Context, trigger string) (result TelegramBackupResult) {
	trigger = normalizeTelegramBackupTrigger(trigger)
	result.Trigger = trigger
	if s == nil {
		result.ErrorClass = "internal"
		return result
	}
	if ctx == nil {
		ctx = context.Background()
		result.ErrorClass = "internal"
		s.recordTelegramBackupRunAudit(telegramBackupActor(ctx, trigger), result)
		return result
	}
	ctx, release, err := acquireOperation(s.Operations, ctx)
	if err != nil {
		result.ErrorClass = "maintenance"
		return result
	}
	defer release()
	actor := telegramBackupActor(ctx, trigger)
	if !telegramBackupRunMu.TryLock() {
		result.ErrorClass = "concurrent_run"
		s.recordTelegramBackupRunAudit(actor, result)
		return result
	}
	defer telegramBackupRunMu.Unlock()
	defer func() {
		if !result.Success && result.ErrorClass == "" {
			result.ErrorClass = "internal"
		}
		s.recordTelegramBackupRunAudit(actor, result)
	}()

	if err := ctx.Err(); err != nil {
		result.ErrorClass = "internal"
		return result
	}
	if s.Settings == nil {
		result.ErrorClass = "settings"
		return result
	}
	telegramEnabled, err := s.Settings.GetTelegramEnabled()
	if err != nil {
		result.ErrorClass = "settings"
		return result
	}
	if !telegramEnabled {
		result.ErrorClass = "disabled"
		return result
	}
	backupEnabled, err := s.Settings.GetTelegramBackupEnabled()
	if err != nil {
		result.ErrorClass = "settings"
		return result
	}
	if !backupEnabled {
		result.ErrorClass = "disabled"
		return result
	}
	token, err := s.Settings.GetTelegramBotToken(ctx)
	if err != nil {
		result.ErrorClass = "settings"
		return result
	}
	if token == "" {
		result.ErrorClass = "missing_token"
		return result
	}
	hasPassphrase, err := s.Settings.HasTelegramBackupPassphrase()
	if err != nil {
		result.ErrorClass = "settings"
		return result
	}
	if !hasPassphrase {
		result.ErrorClass = "missing_passphrase"
		return result
	}
	exclude, err := s.Settings.GetTelegramBackupExcludeTables()
	if err != nil {
		result.ErrorClass = "settings"
		return result
	}
	if s.ParseExcludes == nil {
		result.ErrorClass = "internal"
		return result
	}
	result.ExcludedTables = s.ParseExcludes(exclude)
	maxSizeMB, err := s.Settings.GetTelegramBackupMaxSizeMB()
	if err != nil || maxSizeMB < 1 || maxSizeMB > 50 {
		result.ErrorClass = "settings"
		return result
	}

	if s.Prepare == nil {
		result.ErrorClass = "internal"
		return result
	}
	artifact, errorClass := s.Prepare(ctx, exclude)
	if errorClass != "" {
		result.ErrorClass = errorClass
		return result
	}
	if artifact.Content == nil {
		result.ErrorClass = "internal"
		return result
	}
	defer artifact.Content.Close()
	result.PayloadSizeBytes, result.EnvelopeSizeBytes = artifact.PayloadSizeBytes, artifact.EnvelopeSizeBytes
	envelopeBytes := artifact.EnvelopeSizeBytes

	maxBytes := int64(maxSizeMB) * 1024 * 1024
	if envelopeBytes > maxBytes {
		result.ErrorClass = "oversize"
		return result
	}

	now := time.Now().UTC()
	filename := telegramBackupFilename(now)
	caption := telegramBackupCaption(now, trigger, result.ExcludedTables)
	send := s.SendDocumentStream
	if send == nil && s.Telegram != nil {
		send = s.Telegram.SendDocumentStream
	}
	if send == nil {
		result.ErrorClass = "internal"
		return result
	}
	sendResult := send(ctx, filename, artifact.Content, caption)

	if !sendResult.Success {
		result.ErrorClass = sendResult.ErrorClass
		if result.ErrorClass == "" {
			result.ErrorClass = "internal"
		}
		return result
	}
	result.Success = true
	result.Filename = filename
	return result
}

func normalizeTelegramBackupTrigger(trigger string) string {
	switch trigger {
	case TelegramBackupTriggerScheduled:
		return TelegramBackupTriggerScheduled
	default:
		return TelegramBackupTriggerManual
	}
}

func telegramBackupActor(ctx context.Context, trigger string) string {
	if actor, ok := ctx.Value(telegramBackupActorContextKey{}).(string); ok && actor != "" {
		return actor
	}
	if trigger == TelegramBackupTriggerScheduled {
		return "system"
	}
	return "unknown"
}

func telegramBackupFilename(now time.Time) string {
	return "Solovey UI-backup-" + now.Format("20060102-150405Z") + ".db.aes"
}

func telegramBackupCaption(now time.Time, trigger string, excludedTables []string) string {
	excluded := "none"
	if len(excludedTables) > 0 {
		excluded = strings.Join(excludedTables, ",")
	}
	return redact.String("Solovey UI encrypted database backup\ncreatedAt: " +
		now.Format(time.RFC3339) +
		"\nsource: " + trigger +
		"\nexcludedTables: " + excluded)
}

func (s *TelegramBackupService) recordTelegramBackupRunAudit(actor string, result TelegramBackupResult) {
	details := map[string]any{
		"trigger":           result.Trigger,
		"payloadSizeBytes":  result.PayloadSizeBytes,
		"envelopeSizeBytes": result.EnvelopeSizeBytes,
		"excludedTables":    result.ExcludedTables,
		"channel":           "telegram",
	}
	event := "tg_backup_sent"
	severity := "info"
	if !result.Success {
		event = "tg_backup_failed"
		severity = "warn"
		details["errorClass"] = result.ErrorClass
	}
	if s.Audit == nil {
		return
	}
	if err := s.Audit(AuditRecord{
		Actor:    actor,
		Event:    event,
		Resource: "database",
		Severity: severity,
		Details:  details,
	}); err != nil {
		s.warn("telegram backup audit failed:", err)
	}
}

func (s *TelegramBackupService) warn(args ...any) {
	if s.Warning != nil {
		s.Warning(args...)
	}
}
