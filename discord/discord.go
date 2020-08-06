package discord

import (
	"os"
	"time"

	"github.com/kz/discordrus"
	"github.com/sirupsen/logrus"
)

const (
	webhookURLEnvName = "DISCORD_WEBHOOK_URL"
)

func NewHook(tag, webHookURL string) logrus.Hook {
	return discordrus.NewHook(webHookURL, logrus.ErrorLevel, discordOpts(tag))
}

func discordOpts(tag string) *discordrus.Opts {
	return &discordrus.Opts{
		Username:           tag,
		DisableTimestamp:   false,
		TimestampFormat:    time.RFC3339,
		TimestampLocale:    time.UTC,
		EnableCustomColors: true,
		CustomLevelColors: &discordrus.LevelColors{
			Trace: 3092790,
			Debug: 10170623,
			Info:  3581519,
			Warn:  14327864,
			Error: 13631488,
			Panic: 13631488,
			Fatal: 13631488,
		},
		DisableInlineFields: false, // If set to true, fields will not appear in columns ("inline")
	}
}

func GetWebhookURLFromEnv() string {
	return os.Getenv(webhookURLEnvName)
}
