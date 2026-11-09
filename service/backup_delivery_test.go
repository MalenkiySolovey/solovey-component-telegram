//go:build !minimal

package telegram_test

import (
	"context"
	"io"
	"strings"
	"testing"

	telegram "github.com/MalenkiySolovey/solovey-component-telegram/service"
)

type backupSettings struct {
	deliverySettings
	enabled, passphrase bool
	maximum             int
}

func (s backupSettings) GetTelegramBackupEnabled() (bool, error)         { return s.enabled, nil }
func (s backupSettings) HasTelegramBackupPassphrase() (bool, error)      { return s.passphrase, nil }
func (s backupSettings) GetTelegramBackupExcludeTables() (string, error) { return "stats,changes", nil }
func (s backupSettings) GetTelegramBackupMaxSizeMB() (int, error)        { return s.maximum, nil }

func TestBackupDeliveryUsesAcceptedEncryptedArtifactAndAuditsActualOutcome(t *testing.T) {
	for _, failure := range []string{"", "network", "canceled"} {
		t.Run("outcome-"+failure, func(t *testing.T) {
			var record telegram.AuditRecord
			sent, closed := false, false
			service := &telegram.TelegramBackupService{Settings: backupSettings{deliverySettings{true, "123456:synthetic-token", "42"}, true, true, 45}, Operations: fixtureOperation,
				ParseExcludes: func(string) []string { return []string{"changes", "stats"} },
				Prepare: func(ctx context.Context, exclude string) (telegram.PreparedBackup, string) {
					if ctx == nil || exclude != "stats,changes" {
						t.Fatal("backup owner input changed")
					}
					return telegram.PreparedBackup{Content: &closingBackup{Reader: strings.NewReader("encrypted-fixture"), close: func() { closed = true }}, PayloadSizeBytes: 32, EnvelopeSizeBytes: 64}, ""
				},
				Audit: func(event telegram.AuditRecord) error { record = event; return nil },
				SendDocumentStream: func(ctx context.Context, filename string, source io.Reader, caption string) telegram.Result {
					sent = true
					bytes, err := io.ReadAll(source)
					if err != nil || string(bytes) != "encrypted-fixture" || !strings.HasSuffix(filename, ".db.aes") || !strings.Contains(caption, "source: scheduled") {
						t.Fatal("accepted backup delivery changed")
					}
					return telegram.Result{Success: failure == "", ErrorClass: failure}
				},
			}
			result := service.RunOnce(t.Context(), telegram.TelegramBackupTriggerScheduled)
			if !sent || !closed || result.Success != (failure == "") || result.ErrorClass != failure || record.Actor != "system" {
				t.Fatalf("delivery outcome=%+v audit=%+v closed=%v", result, record, closed)
			}
			expected := "tg_backup_sent"
			if failure != "" {
				expected = "tg_backup_failed"
			}
			if record.Event != expected || record.Details["trigger"] != "scheduled" || record.Details["envelopeSizeBytes"] != int64(64) {
				t.Fatalf("incorrect audit: %+v", record)
			}
		})
	}
}

type closingBackup struct {
	io.Reader
	close func()
}

func (b *closingBackup) Close() error { b.close(); return nil }

func TestBackupDeliveryRejectsMissingPassphraseAndOversizeBeforeUpload(t *testing.T) {
	for _, test := range []struct {
		name       string
		passphrase bool
		size       int64
		class      string
	}{{"missing passphrase", false, 64, "missing_passphrase"}, {"oversize", true, 2 * 1024 * 1024, "oversize"}} {
		t.Run(test.name, func(t *testing.T) {
			prepared, closed := false, false
			service := &telegram.TelegramBackupService{Settings: backupSettings{deliverySettings{true, "123456:synthetic-token", "42"}, true, test.passphrase, 1}, Operations: fixtureOperation, ParseExcludes: func(string) []string { return nil }, Prepare: func(context.Context, string) (telegram.PreparedBackup, string) {
				prepared = true
				return telegram.PreparedBackup{Content: &closingBackup{Reader: strings.NewReader("encrypted"), close: func() { closed = true }}, EnvelopeSizeBytes: test.size}, ""
			}, SendDocumentStream: func(context.Context, string, io.Reader, string) telegram.Result {
				t.Fatal("rejected backup uploaded")
				return telegram.Result{}
			}}
			result := service.RunOnce(t.Context(), "manual")
			if result.Success || result.ErrorClass != test.class || prepared != test.passphrase || closed != test.passphrase {
				t.Fatalf("backup gate=%+v prepared=%v closed=%v", result, prepared, closed)
			}
		})
	}
}
