package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"example.com/devroom/internal/devroom"
	"example.com/devroom/internal/infrai"
)

type server struct {
	realtime *infrai.Client
	now      func() time.Time
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	address := os.Getenv("ADDR")
	if address == "" {
		address = ":8080"
	}
	s := &server{realtime: infrai.NewClient(key), now: time.Now}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rooms/{channel}", s.createRoom)
	mux.HandleFunc("POST /rooms/{channel}/events", s.publishEvent)
	mux.HandleFunc("POST /rooms/{channel}/tokens", s.issueToken)
	mux.HandleFunc("GET /rooms/{channel}/presence", s.getPresence)
	log.Printf("devroomd listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func (s *server) createRoom(w http.ResponseWriter, r *http.Request) {
	channel := r.PathValue("channel")
	data, err := s.realtime.CreateChannel(r.Context(), infrai.ChannelRequest{Channel: channel, Type: "presence"}, "channel:"+channel)
	s.respond(w, data, err, http.StatusCreated)
}

func (s *server) publishEvent(w http.ResponseWriter, r *http.Request) {
	var input devroom.EventInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	event, err := devroom.NormalizeEvent(input, s.now())
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	data, err := s.realtime.Publish(r.Context(), infrai.PublishRequest{
		Channel: r.PathValue("channel"), Event: event.Kind, Data: event, AccountID: event.Actor,
	}, event.ID)
	s.respond(w, data, err, http.StatusAccepted)
}

func (s *server) issueToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ClientID string `json:"client_id"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.ClientID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client_id is required"})
		return
	}
	data, err := s.realtime.IssueToken(r.Context(), infrai.TokenRequest{
		ClientID: input.ClientID, Channels: []string{r.PathValue("channel")},
		Capabilities: []string{"subscribe", "presence"}, TTLSeconds: 900,
	})
	s.respond(w, data, err, http.StatusOK)
}

func (s *server) getPresence(w http.ResponseWriter, r *http.Request) {
	data, err := s.realtime.Presence(r.Context(), r.PathValue("channel"))
	s.respond(w, data, err, http.StatusOK)
}

func (s *server) respond(w http.ResponseWriter, data json.RawMessage, err error, successStatus int) {
	if err == nil {
		writeJSON(w, successStatus, map[string]json.RawMessage{"data": data})
		return
	}
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		writeJSON(w, apiErr.StatusCode, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	log.Printf("realtime request: %v", err)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "realtime request failed"})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
