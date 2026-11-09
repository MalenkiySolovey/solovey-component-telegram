//go:build !minimal

package settings

import (
	"strconv"

	"errors"
	telegramschedule "github.com/MalenkiySolovey/solovey-component-telegram/internal/schedule"
)

// Validate applies the component-owned setting contract. It is shared by the
// runtime contribution and component tests so the accepted values cannot
// diverge between those paths.
func Validate(key string, value string, storedSecretMarker string, validateProxy ...func(string, string) error) error {
	if _, ok := BooleanKeys()[key]; ok {
		if _, err := strconv.ParseBool(value); err != nil {
			return errors.New("invalid boolean setting: " + key)
		}
		return nil
	}
	switch key {
	case CPUThresholdKey:
		threshold, err := strconv.Atoi(value)
		if err != nil || threshold <= 0 || threshold > 100 {
			return errors.New("invalid cpu threshold setting")
		}
	case ReportCronKey, BackupCronKey:
		if _, err := telegramschedule.Parse(value); err != nil {
			return err
		}
	case BackupPassphraseKey:
		if value != "" && value != storedSecretMarker && len([]rune(value)) < 12 {
			return errors.New("weak_passphrase")
		}
	case BackupExcludeTablesKey:
		if len(value) > 256 {
			return errors.New("telegramBackupExcludeTables is too long")
		}
	case BackupMaxSizeMBKey:
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 50 {
			return errors.New("invalid telegram backup max size setting")
		}
	case TransportModeKey:
		if value != "proxy" && value != "outbound" {
			return errors.New("transport mode must be 'proxy' or 'outbound'")
		}
	case OutboundTagKey:
		if len(value) > 256 {
			return errors.New("telegramOutboundTag is too long")
		}
	case ProxyURLKey:
		if value != "" && value != storedSecretMarker {
			if len(validateProxy) == 0 || validateProxy[0] == nil {
				return errors.New("telegram proxy validation unavailable")
			}
			return validateProxy[0](value, storedSecretMarker)
		}
	}
	return nil
}
