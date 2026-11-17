package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godovdm/ai-for-developers-project-386/backend/internal/server"
)

const validOwnerCredential = "correct horse battery staple"

type ownerHarness struct {
	server *httptest.Server
	client *http.Client
	mu     sync.Mutex
	now    time.Time
}

type httpResult struct {
	status  int
	body    string
	header  http.Header
	cookies []*http.Cookie
}

func newOwnerHarness(t *testing.T, password string) *ownerHarness {
	t.Helper()
	return newOwnerHarnessWithAccess(t, server.OwnerAccess{Password: password})
}

func newOwnerHarnessWithAccess(t *testing.T, access server.OwnerAccess) *ownerHarness {
	t.Helper()
	h := &ownerHarness{now: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)}
	handler := server.NewWithDefaultsAndClockAndOwnerAccess(h.clock, access)
	h.server = httptest.NewServer(handler)
	t.Cleanup(h.server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	h.client = &http.Client{Jar: jar}
	return h
}

func (h *ownerHarness) clock() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.now
}

func (h *ownerHarness) advance(duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.now = h.now.Add(duration)
}

func (h *ownerHarness) request(t *testing.T, method, path, body string) httpResult {
	t.Helper()
	return h.requestWithClient(t, h.client, method, path, body, nil)
}

func (h *ownerHarness) requestWithClient(t *testing.T, client *http.Client, method, path, body string, header http.Header) httpResult {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, h.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, values := range header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Fatalf("close response body: %v", err)
		}
	}()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return httpResult{
		status:  response.StatusCode,
		body:    string(payload),
		header:  response.Header.Clone(),
		cookies: response.Cookies(),
	}
}

func (h *ownerHarness) signIn(t *testing.T, credential string) httpResult {
	t.Helper()
	body, err := json.Marshal(map[string]string{"password": credential})
	if err != nil {
		t.Fatalf("marshal login request: %v", err)
	}
	return h.request(t, http.MethodPost, "/api/v1/owner/session", string(body))
}

func TestOwnerSessionDemoModeKeepsOwnerOperationsOpen(t *testing.T) {
	h := newOwnerHarness(t, "")

	session := h.request(t, http.MethodGet, "/api/v1/owner/session", "")
	if session.status != http.StatusOK {
		t.Fatalf("session status = %d, body = %s", session.status, session.body)
	}
	var status struct {
		Required      bool `json:"required"`
		Authenticated bool `json:"authenticated"`
	}
	decodeBody(t, session.body, &status)
	if status.Required || !status.Authenticated {
		t.Errorf("session = %+v, want protection disabled and access open", status)
	}

	created := h.request(t, http.MethodPost, "/api/v1/meeting-types",
		`{"id":"intro","name":"Вводный звонок","description":"","durationMinutes":30}`)
	if created.status != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.status, created.body)
	}
	bookings := h.request(t, http.MethodGet, "/api/v1/bookings", "")
	if bookings.status != http.StatusOK {
		t.Fatalf("bookings status = %d, body = %s", bookings.status, bookings.body)
	}

	signedIn := h.signIn(t, "anything")
	if signedIn.status != http.StatusNoContent {
		t.Fatalf("demo login status = %d, want %d", signedIn.status, http.StatusNoContent)
	}
	if len(signedIn.cookies) != 0 {
		t.Errorf("demo login set cookies = %v, want none", signedIn.cookies)
	}
}

func TestOwnerSessionProtectsOwnerOperations(t *testing.T) {
	h := newOwnerHarness(t, validOwnerCredential)

	session := h.request(t, http.MethodGet, "/api/v1/owner/session", "")
	var status struct {
		Required      bool `json:"required"`
		Authenticated bool `json:"authenticated"`
	}
	decodeBody(t, session.body, &status)
	if !status.Required || status.Authenticated {
		t.Fatalf("session = %+v, want protection enabled and client signed out", status)
	}

	for _, guarded := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/bookings", ""},
		{http.MethodPost, "/api/meeting-types", `{"name":"Тип","description":"Описание","durationMinutes":30}`},
		{http.MethodPost, "/api/v1/meeting-types", `{"id":"intro","name":"Тип","description":"Описание","durationMinutes":30}`},
	} {
		result := h.request(t, guarded.method, guarded.path, guarded.body)
		if result.status != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want %d", guarded.method, guarded.path, result.status, http.StatusUnauthorized)
		}
		expectV1Error(t, result.body, "unauthorized")
	}

	list := h.request(t, http.MethodGet, "/api/v1/meeting-types", "")
	if strings.Contains(list.body, `"intro"`) {
		t.Errorf("rejected request changed state: %s", list.body)
	}

	wrong := h.signIn(t, "nope")
	if wrong.status != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want %d", wrong.status, http.StatusUnauthorized)
	}
	expectV1Error(t, wrong.body, "invalid_credentials")
	if strings.Contains(wrong.body, validOwnerCredential) {
		t.Errorf("response body leaks the owner password: %s", wrong.body)
	}

	signedIn := h.signIn(t, validOwnerCredential)
	if signedIn.status != http.StatusNoContent {
		t.Fatalf("login status = %d, body = %s", signedIn.status, signedIn.body)
	}
	setCookie := signedIn.header.Get("Set-Cookie")
	for _, attribute := range []string{"owner_session=v1.", "Path=/", "HttpOnly", "SameSite=Lax", "Max-Age=43200"} {
		if !strings.Contains(setCookie, attribute) {
			t.Errorf("Set-Cookie = %q, want attribute %q", setCookie, attribute)
		}
	}
	if strings.Contains(setCookie, "Secure") {
		t.Errorf("Set-Cookie = %q, want no Secure over plain HTTP", setCookie)
	}
	if len(signedIn.cookies) != 1 || !strings.HasPrefix(signedIn.cookies[0].Value, "v1.") {
		t.Fatalf("login cookies = %+v, want signed owner_session", signedIn.cookies)
	}
	if expiry := h.clock().Add(12 * time.Hour).Unix(); !strings.HasPrefix(signedIn.cookies[0].Value, "v1."+formatUnix(expiry)+".") {
		t.Errorf("cookie = %q, want expiry %d", signedIn.cookies[0].Value, expiry)
	}

	created := h.request(t, http.MethodPost, "/api/v1/meeting-types",
		`{"id":"intro","name":"Вводный звонок","description":"","durationMinutes":30}`)
	if created.status != http.StatusCreated {
		t.Fatalf("create with session status = %d, body = %s", created.status, created.body)
	}
	bookings := h.request(t, http.MethodGet, "/api/v1/bookings", "")
	if bookings.status != http.StatusOK {
		t.Fatalf("bookings with session status = %d, body = %s", bookings.status, bookings.body)
	}

	logout := h.request(t, http.MethodDelete, "/api/v1/owner/session", "")
	if logout.status != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logout.status, http.StatusNoContent)
	}
	if !strings.Contains(logout.header.Get("Set-Cookie"), "Max-Age=0") {
		t.Errorf("logout Set-Cookie = %q, want cleared cookie", logout.header.Get("Set-Cookie"))
	}
	afterLogout := h.request(t, http.MethodGet, "/api/v1/bookings", "")
	if afterLogout.status != http.StatusUnauthorized {
		t.Fatalf("bookings after logout status = %d, want %d", afterLogout.status, http.StatusUnauthorized)
	}
}

func TestOwnerSessionRejectsForgedAndExpiredCookies(t *testing.T) {
	h := newOwnerHarness(t, validOwnerCredential)
	signedIn := h.signIn(t, validOwnerCredential)
	if signedIn.status != http.StatusNoContent || len(signedIn.cookies) != 1 {
		t.Fatalf("login status = %d, cookies = %v", signedIn.status, signedIn.cookies)
	}
	value := signedIn.cookies[0].Value
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		t.Fatalf("cookie = %q, want three parts", value)
	}

	anonymous := &http.Client{}
	for name, cookie := range map[string]string{
		"garbage":        "garbage",
		"unknown format": "v2.1.abc",
		"forged mac":     parts[0] + "." + parts[1] + ".00" + parts[2][2:],
		"forged expiry":  parts[0] + "." + formatUnix(h.clock().Add(48*time.Hour).Unix()) + "." + parts[2],
	} {
		t.Run(name, func(t *testing.T) {
			result := h.requestWithClient(t, anonymous, http.MethodGet, "/api/v1/bookings", "", http.Header{"Cookie": {"owner_session=" + cookie}})
			if result.status != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d; body = %s", result.status, http.StatusUnauthorized, result.body)
			}
			expectV1Error(t, result.body, "unauthorized")
		})
	}

	h.advance(12*time.Hour + time.Second)
	expired := h.request(t, http.MethodGet, "/api/v1/bookings", "")
	if expired.status != http.StatusUnauthorized {
		t.Errorf("expired session status = %d, want %d", expired.status, http.StatusUnauthorized)
	}
}

func TestOwnerSessionIsInvalidAfterCredentialChange(t *testing.T) {
	first := newOwnerHarness(t, validOwnerCredential)
	signedIn := first.signIn(t, validOwnerCredential)
	if signedIn.status != http.StatusNoContent || len(signedIn.cookies) != 1 {
		t.Fatalf("login status = %d, cookies = %v", signedIn.status, signedIn.cookies)
	}

	changedCredential := validOwnerCredential + "-changed"
	changedSigningKey := validOwnerCredential + "-signing"
	for name, access := range map[string]server.OwnerAccess{
		"new login value": {Password: changedCredential},
		"new signing key": {Password: validOwnerCredential, SessionSecret: changedSigningKey},
	} {
		t.Run(name, func(t *testing.T) {
			second := newOwnerHarnessWithAccess(t, access)
			result := second.requestWithClient(t, &http.Client{}, http.MethodGet, "/api/v1/bookings", "",
				http.Header{"Cookie": {"owner_session=" + signedIn.cookies[0].Value}})
			if result.status != http.StatusUnauthorized {
				t.Errorf("cookie after credential change status = %d, want %d", result.status, http.StatusUnauthorized)
			}
		})
	}
}

func TestOwnerSessionSetsSecureCookieBehindTLSProxy(t *testing.T) {
	h := newOwnerHarness(t, validOwnerCredential)
	loginBody, err := json.Marshal(map[string]string{"password": validOwnerCredential})
	if err != nil {
		t.Fatalf("marshal login request: %v", err)
	}
	secureLogin := h.requestWithClient(t, h.client, http.MethodPost, "/api/v1/owner/session",
		string(loginBody), http.Header{"X-Forwarded-Proto": {"https"}})
	if secureLogin.status != http.StatusNoContent {
		t.Fatalf("login status = %d, body = %s", secureLogin.status, secureLogin.body)
	}
	if !strings.Contains(secureLogin.header.Get("Set-Cookie"), "Secure") {
		t.Errorf("Set-Cookie = %q, want Secure behind a TLS proxy", secureLogin.header.Get("Set-Cookie"))
	}
}

func TestOwnerSessionRejectsCrossOriginWrites(t *testing.T) {
	h := newOwnerHarness(t, validOwnerCredential)
	if signedIn := h.signIn(t, validOwnerCredential); signedIn.status != http.StatusNoContent {
		t.Fatalf("login status = %d, body = %s", signedIn.status, signedIn.body)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.server.URL+"/api/v1/meeting-types",
		strings.NewReader(`{"id":"foreign","name":"Чужой","description":"","durationMinutes":30}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	response, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("cross-origin request: %v", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Fatalf("close body: %v", err)
		}
	}()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want %d; body = %s", response.StatusCode, http.StatusForbidden, payload)
	}
	expectV1Error(t, string(payload), "forbidden_origin")

	list := h.request(t, http.MethodGet, "/api/v1/meeting-types", "")
	if strings.Contains(list.body, `"foreign"`) {
		t.Errorf("cross-origin request changed state: %s", list.body)
	}

	sameOrigin, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.server.URL+"/api/v1/meeting-types",
		strings.NewReader(`{"id":"same-origin","name":"Свой","description":"","durationMinutes":30}`))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	sameOrigin.Header.Set("Content-Type", "application/json")
	sameOrigin.Header.Set("Origin", h.server.URL)
	sameResponse, err := h.client.Do(sameOrigin)
	if err != nil {
		t.Fatalf("same-origin request: %v", err)
	}
	defer func() {
		if err := sameResponse.Body.Close(); err != nil {
			t.Fatalf("close body: %v", err)
		}
	}()
	if sameResponse.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(sameResponse.Body)
		t.Fatalf("same-origin status = %d, want %d; body = %s", sameResponse.StatusCode, http.StatusCreated, payload)
	}
}

func TestPublicOperationsStayOpenWithOwnerProtection(t *testing.T) {
	h := newOwnerHarness(t, validOwnerCredential)

	for _, public := range []struct{ method, path string }{
		{http.MethodGet, "/api/health"},
		{http.MethodGet, "/api/meeting-types"},
		{http.MethodGet, "/api/v1/meeting-types"},
		{http.MethodGet, "/api/v1/meeting-types/meeting-30"},
		{http.MethodGet, "/api/v1/meeting-types/meeting-30/availability"},
	} {
		result := h.request(t, public.method, public.path, "")
		if result.status != http.StatusOK {
			t.Errorf("%s %s status = %d, want %d; body = %s", public.method, public.path, result.status, http.StatusOK, result.body)
		}
	}

	booking := h.request(t, http.MethodPost, "/api/v1/bookings",
		`{"meetingTypeId":"meeting-60","start":"2026-11-16T10:00:00+03:00","guestName":"Гость","guestEmail":"guest@example.com"}`)
	if booking.status != http.StatusCreated {
		t.Errorf("public booking status = %d, want %d; body = %s", booking.status, http.StatusCreated, booking.body)
	}
}

func TestOwnerSessionRejectsMalformedLoginBody(t *testing.T) {
	h := newOwnerHarness(t, validOwnerCredential)
	for _, body := range []string{`{`, `{"password":1}`, `{"password":"x","extra":true}`, `{"password":"x"}{"password":"y"}`} {
		result := h.request(t, http.MethodPost, "/api/v1/owner/session", body)
		if result.status != http.StatusBadRequest {
			t.Errorf("body %q status = %d, want %d", body, result.status, http.StatusBadRequest)
		}
		expectV1Error(t, result.body, "validation_error")
	}
}

func expectV1Error(t *testing.T, body, code string) {
	t.Helper()
	var apiError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decodeBody(t, body, &apiError)
	if apiError.Code != code {
		t.Errorf("error code = %q, want %q; body = %s", apiError.Code, code, body)
	}
	if apiError.Message == "" {
		t.Errorf("error message is empty; body = %s", body)
	}
}

func decodeBody(t *testing.T, body string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), target); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
}

func formatUnix(seconds int64) string {
	return strconv.FormatInt(seconds, 10)
}
