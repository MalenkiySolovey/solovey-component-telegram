//go:build !minimal

// Package telegram delivers administrative notifications using owner-scoped ports.
package telegram

import (
	"context"
	"errors"
	"github.com/MalenkiySolovey/solovey-host/auditrecord"
	transport "github.com/MalenkiySolovey/solovey-host/componentkit/telegram"
	"github.com/MalenkiySolovey/solovey-host/scheduler"
	"net/http"
)

type Settings interface {
	GetTelegramEnabled() (bool, error)
	GetTelegramBotToken(context.Context) (string, error)
	GetTelegramChatID() (string, error)
}

type BackupSettings interface {
	Settings
	GetTelegramBackupEnabled() (bool, error)
	HasTelegramBackupPassphrase() (bool, error)
	GetTelegramBackupExcludeTables() (string, error)
	GetTelegramBackupMaxSizeMB() (int, error)
}

type AuditRecord = auditrecord.Event
type Result = transport.Result

// Service has no ambient DB or network client. The product supplies its existing
// operation lease and approved HTTP client; secrets remain in its settings owner.
type Service struct {
	Settings      Settings
	Operations    scheduler.OperationLease
	ResolveClient func() (*http.Client, bool, error)
	Notifier      *Notifier
	Client        *http.Client
}

func acquireOperation(lease scheduler.OperationLease, ctx context.Context) (context.Context, func(), error) {
	if lease == nil {
		return nil, nil, errors.New("telegram operation admission unavailable")
	}
	admitted, release, err := lease(ctx)
	if err != nil || admitted == nil || release == nil {
		if release != nil {
			release()
		}
		return nil, nil, errors.New("telegram operation admission unavailable")
	}
	return admitted, release, nil
}

func (s *Service) httpClient() (*http.Client, bool, error) {
	if s.Client != nil {
		return s.Client, false, nil
	}
	if s.ResolveClient == nil {
		return nil, false, errors.New("telegram HTTP client unavailable")
	}
	return s.ResolveClient()
}
