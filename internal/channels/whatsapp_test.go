package channels

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coopco/nanobot/internal/bus"
)

func newTestWhatsApp(t *testing.T, allowedUsers []string) *WhatsAppChannel {
	t.Helper()
	cfg := whatsAppConfig{
		AccessToken:   "test-token",
		PhoneNumberID: "12345",
		VerifyToken:   "secret",
		WebhookPort:   0,
		AllowedUsers:  allowedUsers,
	}
	raw, _ := json.Marshal(cfg)
	msgBus := bus.NewMessageBus(16)
	ch, err := newWhatsAppChannel(raw, msgBus)
	if err != nil {
		t.Fatalf("newWhatsAppChannel: %v", err)
	}
	return ch.(*WhatsAppChannel)
}

func TestWhatsAppWebhookVerifyCorrectToken(t *testing.T) {
	ch := newTestWhatsApp(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=secret&hub.challenge=mychallenge", nil)
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "mychallenge" {
		t.Errorf("expected challenge %q, got %q", "mychallenge", string(body))
	}
}

func TestWhatsAppWebhookVerifyWrongToken(t *testing.T) {
	ch := newTestWhatsApp(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=mychallenge", nil)
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestWhatsAppWebhookVerifyWrongMode(t *testing.T) {
	ch := newTestWhatsApp(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=other&hub.verify_token=secret&hub.challenge=x", nil)
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestWhatsAppIncomingMessage(t *testing.T) {
	msgBus := bus.NewMessageBus(16)
	cfg := whatsAppConfig{
		AccessToken:   "tok",
		PhoneNumberID: "pid",
		VerifyToken:   "v",
		WebhookPort:   0,
	}
	raw, _ := json.Marshal(cfg)
	ch, err := newWhatsAppChannel(raw, msgBus)
	if err != nil {
		t.Fatalf("newWhatsAppChannel: %v", err)
	}
	wa := ch.(*WhatsAppChannel)

	payload := `{
		"entry": [{
			"changes": [{
				"value": {
					"messages": [{
						"from": "15551234567",
						"id": "msg1",
						"type": "text",
						"text": {"body": "hello world"}
					}]
				}
			}]
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	w := httptest.NewRecorder()
	wa.handleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	received, err := msgBus.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("expected inbound message, got error: %v", err)
	}
	if received.Content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", received.Content)
	}
	if received.SenderID != "15551234567" {
		t.Errorf("expected senderID %q, got %q", "15551234567", received.SenderID)
	}
	if received.Channel != "whatsapp" {
		t.Errorf("expected channel %q, got %q", "whatsapp", received.Channel)
	}
}

func TestWhatsAppIncomingNonTextIgnored(t *testing.T) {
	msgBus := bus.NewMessageBus(16)
	cfg := whatsAppConfig{AccessToken: "tok", PhoneNumberID: "pid", VerifyToken: "v"}
	raw, _ := json.Marshal(cfg)
	ch, _ := newWhatsAppChannel(raw, msgBus)
	wa := ch.(*WhatsAppChannel)

	payload := `{
		"entry": [{
			"changes": [{
				"value": {
					"messages": [{
						"from": "123",
						"id": "m1",
						"type": "image",
						"text": {"body": ""}
					}]
				}
			}]
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	w := httptest.NewRecorder()
	wa.handleWebhook(w, req)

	// nothing should be on the inbound channel
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := msgBus.ConsumeInbound(ctx)
	if err == nil {
		t.Error("expected no inbound message for non-text type")
	}
}

func TestWhatsAppIncomingInvalidJSON(t *testing.T) {
	ch := newTestWhatsApp(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWhatsAppIsAllowedEmptyList(t *testing.T) {
	ch := newTestWhatsApp(t, nil)
	if !ch.IsAllowed("anyone") {
		t.Error("expected IsAllowed=true with empty allowlist")
	}
}

func TestWhatsAppIsAllowedWithList(t *testing.T) {
	ch := newTestWhatsApp(t, []string{"alice", "bob"})
	if !ch.IsAllowed("alice") {
		t.Error("expected alice to be allowed")
	}
	if ch.IsAllowed("charlie") {
		t.Error("expected charlie to be denied")
	}
}

func TestWhatsAppSendMockServer(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Build a channel that points at our mock server by patching the URL via
	// a custom http.Client — but WhatsAppChannel uses http.DefaultClient and
	// a hardcoded URL. Instead we test the error path (non-200 response).
	// For the happy path we verify the request shape via a round-tripper.
	msgBus := bus.NewMessageBus(16)
	cfg := whatsAppConfig{
		AccessToken:   "Bearer-tok",
		PhoneNumberID: "PHONE_ID",
		VerifyToken:   "v",
	}
	raw, _ := json.Marshal(cfg)
	ch, _ := newWhatsAppChannel(raw, msgBus)
	wa := ch.(*WhatsAppChannel)

	// Swap DefaultTransport temporarily to redirect to our test server.
	origTransport := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: srv.URL, base: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	err := wa.Send(bus.OutboundMessage{ChatID: "dest123", Content: "hi there"})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if gotAuth != "Bearer Bearer-tok" {
		t.Errorf("expected Authorization header %q, got %q", "Bearer Bearer-tok", gotAuth)
	}
	if !strings.Contains(gotBody, "hi there") {
		t.Errorf("expected body to contain message content, got %q", gotBody)
	}
}

// redirectTransport rewrites the host of every request to the given target.
type redirectTransport struct {
	target string
	base   http.RoundTripper
}

func (r *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Host = strings.TrimPrefix(r.target, "http://")
	req2.URL.Scheme = "http"
	return r.base.RoundTrip(req2)
}

func TestWhatsAppSendNon200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	msgBus := bus.NewMessageBus(16)
	cfg := whatsAppConfig{AccessToken: "tok", PhoneNumberID: "pid", VerifyToken: "v"}
	raw, _ := json.Marshal(cfg)
	ch, _ := newWhatsAppChannel(raw, msgBus)
	wa := ch.(*WhatsAppChannel)

	origTransport := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: srv.URL, base: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	err := wa.Send(bus.OutboundMessage{ChatID: "dest", Content: "msg"})
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestWhatsAppName(t *testing.T) {
	ch := newTestWhatsApp(t, nil)
	if ch.Name() != "whatsapp" {
		t.Errorf("expected name %q, got %q", "whatsapp", ch.Name())
	}
}

func TestWhatsAppDisallowedUserIgnored(t *testing.T) {
	msgBus := bus.NewMessageBus(16)
	cfg := whatsAppConfig{
		AccessToken:   "tok",
		PhoneNumberID: "pid",
		VerifyToken:   "v",
		AllowedUsers:  []string{"allowed-user"},
	}
	raw, _ := json.Marshal(cfg)
	ch, _ := newWhatsAppChannel(raw, msgBus)
	wa := ch.(*WhatsAppChannel)

	payload := `{
		"entry": [{
			"changes": [{
				"value": {
					"messages": [{
						"from": "blocked-user",
						"id": "m1",
						"type": "text",
						"text": {"body": "hello"}
					}]
				}
			}]
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	w := httptest.NewRecorder()
	wa.handleWebhook(w, req)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := msgBus.ConsumeInbound(ctx)
	if err == nil {
		t.Error("expected no inbound message for disallowed user")
	}
}
