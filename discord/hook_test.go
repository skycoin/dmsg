package discord

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHook_shouldFire(t *testing.T) {
	hook := &Hook{
		parent:     nil,
		limit:      1 * time.Millisecond,
		timestamps: make(map[string]time.Time),
	}

	ts := time.Now()

	tests := []struct {
		name      string
		message   string
		timestamp time.Time
		want      bool
	}{
		{
			name:      "Case 1",
			message:   "Message 1",
			timestamp: ts,
			want:      true,
		},
		{
			name:      "Case 2",
			message:   "Message 2",
			timestamp: ts,
			want:      true,
		},
		{
			name:      "Case 3",
			message:   "Message 1",
			timestamp: ts,
			want:      false,
		},
		{
			name:      "Case 4",
			message:   "Message 1",
			timestamp: ts.Add(500 * time.Microsecond),
			want:      false,
		},
		{
			name:      "Case 5",
			message:   "Message 1",
			timestamp: ts.Add(1500 * time.Microsecond),
			want:      true,
		},
		{
			name:      "Case 6",
			message:   "Message 1",
			timestamp: ts.Add(2000 * time.Microsecond),
			want:      false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			entry := &logrus.Entry{
				Time:    tt.timestamp,
				Message: tt.message,
			}

			assert.Equal(t, tt.want, hook.shouldFire(entry))
		})
	}
}
