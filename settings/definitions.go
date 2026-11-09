//go:build !minimal

// Package settings defines Telegram settings without owning their persistence.
package settings

const (
	EnabledKey             = "telegramEnabled"
	BotTokenKey            = "telegramBotToken"
	ChatIDKey              = "telegramChatID"
	ProxyURLKey            = "telegramProxyURL"
	ProxyUsernameKey       = "telegramProxyUsername"
	ProxyPasswordKey       = "telegramProxyPassword"
	TransportModeKey       = "telegramTransportMode"
	OutboundTagKey         = "telegramOutboundTag"
	CPUThresholdKey        = "telegramCpuThreshold"
	NotifyCPUKey           = "telegramNotifyCpu"
	ReportKey              = "telegramReport"
	ReportCronKey          = "telegramReportCron"
	BackupEnabledKey       = "telegramBackupEnabled"
	BackupPassphraseKey    = "telegramBackupPassphrase"
	BackupCronKey          = "telegramBackupCron"
	BackupExcludeTablesKey = "telegramBackupExcludeTables"
	BackupMaxSizeMBKey     = "telegramBackupMaxSizeMB"
)

func Defaults() map[string]string {
	return map[string]string{
		EnabledKey:             "false",
		BotTokenKey:            "",
		ChatIDKey:              "",
		ProxyURLKey:            "",
		ProxyUsernameKey:       "",
		ProxyPasswordKey:       "",
		TransportModeKey:       "proxy",
		OutboundTagKey:         "",
		CPUThresholdKey:        "90",
		NotifyCPUKey:           "false",
		ReportKey:              "false",
		ReportCronKey:          "",
		BackupEnabledKey:       "false",
		BackupPassphraseKey:    "",
		BackupCronKey:          "",
		BackupExcludeTablesKey: "stats,client_ips,audit_events,changes",
		BackupMaxSizeMBKey:     "45",
	}
}

func AllKeys() []string {
	keys := make([]string, 0, len(Defaults()))
	for key := range Defaults() {
		keys = append(keys, key)
	}
	return keys
}

func BooleanKeys() map[string]struct{} {
	return keySet(
		EnabledKey,
		NotifyCPUKey,
		ReportKey,
		BackupEnabledKey,
	)
}

func EncryptedKeys() map[string]struct{} {
	return keySet(
		BackupPassphraseKey,
		BotTokenKey,
		ProxyPasswordKey,
		ProxyURLKey,
		ProxyUsernameKey,
	)
}

func ClearableEmptyEncryptedKeys() map[string]struct{} {
	return keySet(BackupPassphraseKey)
}

func keySet(keys ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set
}
