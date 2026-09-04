package devroom

import (
	"testing"
	"time"
)

func TestNormalizeEventSelectsDeveloperSeverity(t *testing.T) {
	now := time.Date(2026, time.September, 2, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		kind string
		want string
	}{
		{kind: "build.completed", want: "info"},
		{kind: "release.started", want: "notice"},
		{kind: "diagnostic.opened", want: "warning"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			event, err := NormalizeEvent(EventInput{ID: "evt-42", Kind: tt.kind, Summary: "linux build finished"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if event.Severity != tt.want {
				t.Fatalf("severity = %q, want %q", event.Severity, tt.want)
			}
			if event.OccurredAt != "2026-09-02T10:00:00Z" {
				t.Fatalf("occurred_at = %q", event.OccurredAt)
			}
		})
	}
}
