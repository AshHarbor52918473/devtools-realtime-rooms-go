package devroom

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type EventInput struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	Actor     string `json:"actor"`
	Reference string `json:"reference"`
}

type RoomEvent struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Summary    string `json:"summary"`
	Actor      string `json:"actor"`
	Reference  string `json:"reference,omitempty"`
	Severity   string `json:"severity"`
	OccurredAt string `json:"occurred_at"`
}

func NormalizeEvent(input EventInput, now time.Time) (RoomEvent, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Summary = strings.TrimSpace(input.Summary)
	if input.ID == "" || input.Summary == "" {
		return RoomEvent{}, errors.New("id and summary are required")
	}
	severity := map[string]string{
		"build.completed":   "info",
		"release.started":   "notice",
		"diagnostic.opened": "warning",
	}[input.Kind]
	if severity == "" {
		return RoomEvent{}, fmt.Errorf("unsupported event kind %q", input.Kind)
	}
	return RoomEvent{
		ID: input.ID, Kind: input.Kind, Summary: input.Summary,
		Actor: input.Actor, Reference: input.Reference, Severity: severity,
		OccurredAt: now.UTC().Format(time.RFC3339),
	}, nil
}
