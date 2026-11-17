package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/godovdm/ai-for-developers-project-386/backend/internal/api/generated"
)

const (
	ownerCookieName = "owner_session"
	ownerSessionTTL = 12 * time.Hour
	ownerSessionV1  = "v1"
	ownerKeyLabel   = "calendar-owner-session-v1"
)

// OwnerAccess configures the owner password and the key that signs sessions.
// An empty Password keeps the server in the demonstration mode: owner
// operations stay open and no default password exists.
type OwnerAccess struct {
	Password      string
	SessionSecret string
}

// Enabled reports whether owner operations require a session.
func (access OwnerAccess) Enabled() bool {
	return access.Password != ""
}

func (access OwnerAccess) signingKey() []byte {
	if access.SessionSecret != "" {
		return []byte(access.SessionSecret)
	}
	mac := hmac.New(sha256.New, []byte(access.Password))
	mac.Write([]byte(ownerKeyLabel))
	return mac.Sum(nil)
}

// ownerOperations is the single list of routes that require an owner session.
var ownerOperations = map[string]struct{}{
	http.MethodPost + " /api/meeting-types":    {},
	http.MethodPost + " /api/v1/meeting-types": {},
	http.MethodGet + " /api/v1/bookings":       {},
}

func (s server) ownerGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.owner.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		if _, guarded := ownerOperations[r.Method+" "+r.URL.Path]; !guarded {
			next.ServeHTTP(w, r)
			return
		}
		if !sameOrigin(r) {
			writeV1Error(w, http.StatusForbidden, "forbidden_origin", "Запрос с другого источника отклонён")
			return
		}
		if !s.validSession(r, s.now()) {
			writeV1Error(w, http.StatusUnauthorized, "unauthorized", "Требуется вход владельца")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request) bool {
	source := r.Header.Get("Origin")
	if source == "" {
		source = r.Header.Get("Referer")
	}
	if source == "" {
		return true
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func (s server) validSession(r *http.Request, now time.Time) bool {
	cookie, err := r.Cookie(ownerCookieName)
	if err != nil {
		return false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 || parts[0] != ownerSessionV1 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() >= expiry {
		return false
	}
	return hmac.Equal([]byte(s.sessionMAC(parts[0]+"."+parts[1])), []byte(parts[2]))
}

func (s server) sessionMAC(payload string) string {
	mac := hmac.New(sha256.New, s.owner.signingKey())
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s server) newSessionValue(now time.Time) string {
	expiry := now.Add(ownerSessionTTL)
	payload := ownerSessionV1 + "." + strconv.FormatInt(expiry.Unix(), 10)
	return payload + "." + s.sessionMAC(payload)
}

func passwordMatches(got, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func requestIsTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s server) setSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	cookie := &http.Cookie{
		Name:     ownerCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if requestIsTLS(r) {
		cookie.Secure = true
	}
	http.SetCookie(w, cookie)
}

func (s server) GetOwnerSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, generated.OwnerSessionStatus{
		Required:      s.owner.Enabled(),
		Authenticated: !s.owner.Enabled() || s.validSession(r, s.now()),
	})
}

func (s server) CreateOwnerSession(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("close request body: %v", err)
		}
	}()
	var request generated.OwnerLoginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeV1Error(w, http.StatusBadRequest, "validation_error", "Некорректный запрос")
		return
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeV1Error(w, http.StatusBadRequest, "validation_error", "Некорректный запрос")
		return
	}
	if !s.owner.Enabled() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !passwordMatches(request.Password, s.owner.Password) {
		writeV1Error(w, http.StatusUnauthorized, "invalid_credentials", "Неверный пароль")
		return
	}
	value := s.newSessionValue(s.now())
	s.setSessionCookie(w, r, value, int(ownerSessionTTL.Seconds()))
	w.WriteHeader(http.StatusNoContent)
}

func (s server) DeleteOwnerSession(w http.ResponseWriter, r *http.Request) {
	s.setSessionCookie(w, r, "", -1)
	w.WriteHeader(http.StatusNoContent)
}
