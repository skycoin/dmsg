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

type Hook struct {
	parent     logrus.Hook
	limit      time.Duration
	timestamps map[string]time.Time
}

type Option func(*Hook)

// WithLimit sets enables logger rate limiter with specified limit.
func WithLimit(limit time.Duration) Option {
	return func(h *Hook) {
		h.limit = limit
		h.timestamps = make(map[string]time.Time)
	}
}

// NewHook returns a new Hook.
func NewHook(tag, webHookURL string, opts ...Option) logrus.Hook {
	parent := discordrus.NewHook(webHookURL, logrus.ErrorLevel, discordOpts(tag))

	hook := &Hook{
		parent: parent,
	}

	for _, opt := range opts {
		opt(hook)
	}

	return hook
}

func (h *Hook) Levels() []logrus.Level {
	return h.parent.Levels()
}

func (h *Hook) Fire(entry *logrus.Entry) error {
	if h.shouldFire(entry) {
		return h.parent.Fire(entry)
	}

	return nil
}

func (h *Hook) shouldFire(entry *logrus.Entry) bool {
	if h.limit != 0 && h.timestamps != nil {
		v, ok := h.timestamps[entry.Message]
		if ok && entry.Time.Sub(v) < h.limit {
			return false
		}

		h.timestamps[entry.Message] = entry.Time
	}

	return true
}

func discordOpts(tag string) *discordrus.Opts {
	return &discordrus.Opts{
		Username:        tag,
		TimestampFormat: time.RFC3339,
		TimestampLocale: time.UTC,
	}
}

func GetWebhookURLFromEnv() string {
	return os.Getenv(webhookURLEnvName)
}
