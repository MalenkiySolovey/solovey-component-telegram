//go:build !minimal

package settings

const (
	telegramSettingsPage = "telegram"
	telegramCoreGroup    = "telegram_core"
	telegramBackupGroup  = "telegram_backup"
)

// Field is a Telegram setting contribution; the product adapts its schema type.
type Field struct {
	Key, Page, Group, Type, LabelKey string
	Order                            int
	Advanced                         bool
	Options                          []string
	Min, Max                         *int
}

func Fields() []Field {
	return []Field{
		{Key: EnabledKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "bool", LabelKey: "telegram.enabled", Order: 10},
		{Key: BotTokenKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "secret", LabelKey: "telegram.botToken", Order: 20},
		{Key: ChatIDKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "string", LabelKey: "telegram.chatId", Order: 30},
		{Key: CPUThresholdKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "int", LabelKey: "telegram.cpuThreshold", Min: intPtr(1), Max: intPtr(100), Order: 40},
		{Key: NotifyCPUKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "bool", LabelKey: "telegram.notifyCpu", Order: 50},
		{Key: ReportKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "bool", LabelKey: "telegram.report", Order: 60},
		{Key: ReportCronKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "cron", LabelKey: "telegram.reportCron", Order: 70},
		{Key: TransportModeKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "enum", LabelKey: "telegram.transport", Options: []string{"proxy", "outbound"}, Order: 80},
		{Key: OutboundTagKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "string", LabelKey: "telegram.outbound", Order: 90},
		{Key: ProxyURLKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "secret", LabelKey: "telegram.proxyUrl", Advanced: true, Order: 100},
		{Key: ProxyUsernameKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "secret", LabelKey: "telegram.proxyUsername", Advanced: true, Order: 110},
		{Key: ProxyPasswordKey, Page: telegramSettingsPage, Group: telegramCoreGroup, Type: "secret", LabelKey: "telegram.proxyPassword", Advanced: true, Order: 120},

		{Key: BackupEnabledKey, Page: telegramSettingsPage, Group: telegramBackupGroup, Type: "bool", LabelKey: "telegram.backup.enabled", Order: 10},
		{Key: BackupPassphraseKey, Page: telegramSettingsPage, Group: telegramBackupGroup, Type: "secret", LabelKey: "telegram.backup.passphrase", Order: 20},
		{Key: BackupCronKey, Page: telegramSettingsPage, Group: telegramBackupGroup, Type: "cron", LabelKey: "telegram.backup.schedule.title", Order: 30},
		{Key: BackupExcludeTablesKey, Page: telegramSettingsPage, Group: telegramBackupGroup, Type: "tag_list", LabelKey: "telegram.backup.excludeTables", Options: []string{"stats", "client_ips", "audit_events", "changes"}, Order: 40},
		{Key: BackupMaxSizeMBKey, Page: telegramSettingsPage, Group: telegramBackupGroup, Type: "int", LabelKey: "telegram.backup.maxSize", Min: intPtr(1), Max: intPtr(50), Order: 50},
	}
}

func intPtr(value int) *int {
	return &value
}

func ImportAliases() map[string]string {
	return map[string]string{"tgBotEnable": EnabledKey, "tgBotToken": BotTokenKey, "tgBotChatId": ChatIDKey, "tgRunTime": ReportCronKey, "tgCpu": CPUThresholdKey, "tgBotBackup": BackupEnabledKey, "tgBotProxy": ProxyURLKey}
}
