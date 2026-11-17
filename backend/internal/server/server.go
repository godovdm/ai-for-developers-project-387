// Package server implements the calendar HTTP API.
package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/godovdm/ai-for-developers-project-386/backend/internal/api/generated"
	"github.com/godovdm/ai-for-developers-project-386/backend/internal/meeting"
)

const maxRequestBodySize = 1 << 20

type createMeetingTypeRequest struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	DurationMinutes json.RawMessage `json:"durationMinutes"`
}

type server struct {
	meetingTypes meeting.Repository
	v1           *meeting.Memory
	now          func() time.Time
	owner        OwnerAccess
}

// New returns the calendar API handler.
func New() http.Handler {
	return NewWithClock(time.Now)
}

// NewWithDefaults returns the calendar handler with built-in meeting types.
func NewWithDefaults() http.Handler {
	return NewWithOwnerAccess(OwnerAccess{})
}

// NewWithOwnerAccess returns the handler with built-in meeting types and the
// supplied owner access configuration. An empty password keeps the
// demonstration mode.
func NewWithOwnerAccess(access OwnerAccess) http.Handler {
	return NewWithDefaultsAndClockAndOwnerAccess(time.Now, access)
}

// NewWithDefaultsAndClockAndOwnerAccess returns the handler with built-in
// meeting types, the supplied clock and owner access configuration.
func NewWithDefaultsAndClockAndOwnerAccess(now func() time.Time, access OwnerAccess) http.Handler {
	store := meeting.NewMemory()
	for _, meetingType := range []meeting.Type{
		{
			ID:          "meeting-30",
			Name:        "Встреча на 30 минут",
			Description: "Короткая встреча для знакомства и обсуждения вопросов",
			Duration:    meeting.DurationThirty,
		},
		{
			ID:          "meeting-60",
			Name:        "Встреча на 60 минут",
			Description: "Подробная встреча для глубокого обсуждения",
			Duration:    meeting.DurationSixty,
		},
	} {
		if !store.CreateWithID(meetingType) {
			panic("duplicate built-in meeting type ID: " + meetingType.ID)
		}
	}

	return newHandler(
		store,
		now,
		access,
	)
}

// NewWithClock returns the calendar handler using the supplied clock.
func NewWithClock(now func() time.Time) http.Handler {
	return NewWithClockAndOwnerAccess(now, OwnerAccess{})
}

// NewWithClockAndOwnerAccess returns the calendar handler using the supplied
// clock and owner access configuration.
func NewWithClockAndOwnerAccess(now func() time.Time, access OwnerAccess) http.Handler {
	store := meeting.NewMemory()
	return newHandler(
		store,
		now,
		access,
	)
}

func newHandler(store *meeting.Memory, now func() time.Time, access OwnerAccess) http.Handler {
	s := server{
		meetingTypes: store,
		v1:           store,
		now:          now,
		owner:        access,
	}
	return s.ownerGuard(generated.Handler(s))
}

func (server) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, generated.HealthStatus{Status: "ok"})
}

func (s server) ListMeetingTypes(w http.ResponseWriter, _ *http.Request) {
	items := s.meetingTypes.List()
	meetingTypes := make([]generated.MeetingType, 0, len(items))
	for _, item := range items {
		meetingTypes = append(meetingTypes, toAPIMeetingType(item))
	}
	writeJSON(w, http.StatusOK, meetingTypes)
}

func (s server) CreateMeetingType(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("close request body: %v", err)
		}
	}()

	var request createMeetingTypeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	if err := decoder.Decode(&request); err != nil {
		writeError(w, "Некорректный запрос")
		return
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(w, "Некорректный запрос")
		return
	}

	duration, ok := parseDuration(request.DurationMinutes)
	if !ok {
		writeError(w, meeting.MessageInvalidDuration)
		return
	}

	meetingType, err := meeting.NewType(request.Name, request.Description, duration)
	if err != nil {
		var validationError *meeting.ValidationError
		if errors.As(err, &validationError) {
			writeError(w, validationError.Message)
			return
		}
		writeError(w, "Некорректные данные типа встречи")
		return
	}

	created := s.meetingTypes.Create(meetingType)
	writeJSON(w, http.StatusCreated, toAPIMeetingType(created))
}

func parseDuration(raw json.RawMessage) (meeting.Duration, bool) {
	token := bytes.TrimSpace(raw)
	if len(token) == 0 || (token[0] != '-' && (token[0] < '0' || token[0] > '9')) {
		return 0, false
	}
	value, ok := new(big.Rat).SetString(string(token))
	if !ok {
		return 0, false
	}
	switch {
	case value.Cmp(big.NewRat(30, 1)) == 0:
		return meeting.DurationThirty, true
	case value.Cmp(big.NewRat(60, 1)) == 0:
		return meeting.DurationSixty, true
	case value.Cmp(big.NewRat(90, 1)) == 0:
		return meeting.DurationNinety, true
	case value.Cmp(big.NewRat(120, 1)) == 0:
		return meeting.DurationOneHundredTwenty, true
	default:
		return 0, false
	}
}

func toAPIMeetingType(meetingType meeting.Type) generated.MeetingType {
	return generated.MeetingType{
		Id:              meetingType.ID,
		Name:            meetingType.Name,
		Description:     meetingType.Description,
		DurationMinutes: generated.MeetingDuration(meetingType.Duration),
	}
}

func writeError(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, generated.ApiError{Message: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
