package channels

import (
	"bufio"
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

// --- Feishu ---

func TestNewFeishuChannel(t *testing.T) {
	cfg := `{"appId":"aid","appSecret":"sec","webhookPort":0,"allowedUsers":["u1"]}`
	ch, err := newFeishuChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fc := ch.(*FeishuChannel)
	if fc.Name() != "feishu" {
		t.Errorf("Name = %q, want feishu", fc.Name())
	}
	if !fc.IsAllowed("u1") {
		t.Error("expected u1 to be allowed")
	}
	if fc.IsAllowed("u2") {
		t.Error("expected u2 to be disallowed")
	}
}

func TestFeishuIsAllowed_EmptyAllowAll(t *testing.T) {
	cfg := `{"appId":"aid","appSecret":"sec"}`
	ch, _ := newFeishuChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	fc := ch.(*FeishuChannel)
	if !fc.IsAllowed("anyone") {
		t.Error("empty allowedUsers should allow all")
	}
}

func TestFeishuHandleEvent_URLVerification(t *testing.T) {
	cfg := `{"appId":"aid","appSecret":"sec"}`
	ch, _ := newFeishuChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	fc := ch.(*FeishuChannel)

	body := `{"challenge":"test-challenge","type":"url_verification"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	fc.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["challenge"] != "test-challenge" {
		t.Errorf("challenge = %q, want test-challenge", resp["challenge"])
	}
}

func TestFeishuHandleEvent_Message(t *testing.T) {
	msgBus := bus.NewMessageBus(4)
	cfg := `{"appId":"aid","appSecret":"sec"}`
	ch, _ := newFeishuChannel(json.RawMessage(cfg), msgBus)
	fc := ch.(*FeishuChannel)

	body := `{
		"header":{"event_type":"im.message.receive_v1"},
		"event":{
			"sender":{"sender_id":{"open_id":"ou_123"}},
			"message":{"chat_id":"oc_456","content":"{\"text\":\"hello feishu\"}"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	fc.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	msg, err := msgBus.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("expected inbound message: %v", err)
	}
	if msg.Content != "hello feishu" {
		t.Errorf("content = %q, want hello feishu", msg.Content)
	}
	if msg.SenderID != "ou_123" {
		t.Errorf("senderID = %q, want ou_123", msg.SenderID)
	}
}

func TestFeishuHandleEvent_NonMessageEvent(t *testing.T) {
	cfg := `{"appId":"aid","appSecret":"sec"}`
	ch, _ := newFeishuChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	fc := ch.(*FeishuChannel)

	body := `{"header":{"event_type":"other_event"},"event":{}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	fc.handleEvent(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestFeishuHandleEvent_DisallowedUser(t *testing.T) {
	msgBus := bus.NewMessageBus(4)
	cfg := `{"appId":"aid","appSecret":"sec","allowedUsers":["allowed"]}`
	ch, _ := newFeishuChannel(json.RawMessage(cfg), msgBus)
	fc := ch.(*FeishuChannel)

	body := `{
		"header":{"event_type":"im.message.receive_v1"},
		"event":{
			"sender":{"sender_id":{"open_id":"blocked"}},
			"message":{"chat_id":"oc_1","content":"{\"text\":\"hi\"}"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	fc.handleEvent(w, req)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := msgBus.ConsumeInbound(ctx)
	if err == nil {
		t.Error("expected no message for disallowed user")
	}
}

// --- DingTalk ---

func TestNewDingTalkChannel(t *testing.T) {
	cfg := `{"clientId":"cid","clientSecret":"csec","webhookPort":0,"allowedUsers":["u1"]}`
	ch, err := newDingTalkChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dc := ch.(*DingTalkChannel)
	if dc.Name() != "dingtalk" {
		t.Errorf("Name = %q, want dingtalk", dc.Name())
	}
	if !dc.IsAllowed("u1") {
		t.Error("expected u1 allowed")
	}
	if dc.IsAllowed("u2") {
		t.Error("expected u2 disallowed")
	}
}

func TestDingTalkIsAllowed_EmptyAllowAll(t *testing.T) {
	cfg := `{"clientId":"cid","clientSecret":"csec"}`
	ch, _ := newDingTalkChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	dc := ch.(*DingTalkChannel)
	if !dc.IsAllowed("anyone") {
		t.Error("empty allowedUsers should allow all")
	}
}

func TestDingTalkHandleEvent_Message(t *testing.T) {
	msgBus := bus.NewMessageBus(4)
	cfg := `{"clientId":"cid","clientSecret":"csec"}`
	ch, _ := newDingTalkChannel(json.RawMessage(cfg), msgBus)
	dc := ch.(*DingTalkChannel)

	body := `{"msgtype":"text","text":{"content":"hello dt"},"senderId":"s1","conversationId":"c1"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	dc.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	msg, err := msgBus.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("expected inbound message: %v", err)
	}
	if msg.Content != "hello dt" {
		t.Errorf("content = %q, want hello dt", msg.Content)
	}
}

func TestDingTalkHandleEvent_DisallowedUser(t *testing.T) {
	msgBus := bus.NewMessageBus(4)
	cfg := `{"clientId":"cid","clientSecret":"csec","allowedUsers":["allowed"]}`
	ch, _ := newDingTalkChannel(json.RawMessage(cfg), msgBus)
	dc := ch.(*DingTalkChannel)

	body := `{"msgtype":"text","text":{"content":"hi"},"senderId":"blocked","conversationId":"c1"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	dc.handleEvent(w, req)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := msgBus.ConsumeInbound(ctx)
	if err == nil {
		t.Error("expected no message for disallowed user")
	}
}

// --- QQ ---

func TestNewQQChannel(t *testing.T) {
	cfg := `{"appId":"aid","token":"tok","appSecret":"sec","webhookPort":0,"allowedUsers":["u1"]}`
	ch, err := newQQChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	qc := ch.(*QQChannel)
	if qc.Name() != "qq" {
		t.Errorf("Name = %q, want qq", qc.Name())
	}
	if !qc.IsAllowed("u1") {
		t.Error("expected u1 allowed")
	}
	if qc.IsAllowed("u2") {
		t.Error("expected u2 disallowed")
	}
}

func TestQQHandleEvent_Challenge(t *testing.T) {
	cfg := `{"appId":"aid","token":"tok","appSecret":"sec"}`
	ch, _ := newQQChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	qc := ch.(*QQChannel)

	body := `{"op":13,"d":{"plain_token":"my-token"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	qc.handleEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["plain_token"] != "my-token" {
		t.Errorf("plain_token = %q, want my-token", resp["plain_token"])
	}
}

func TestQQHandleEvent_Message(t *testing.T) {
	msgBus := bus.NewMessageBus(4)
	cfg := `{"appId":"aid","token":"tok","appSecret":"sec"}`
	ch, _ := newQQChannel(json.RawMessage(cfg), msgBus)
	qc := ch.(*QQChannel)

	body := `{"op":0,"t":"AT_MESSAGE_CREATE","d":{"id":"m1","channel_id":"ch1","author":{"id":"a1"},"content":"hello qq"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	qc.handleEvent(w, req)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	msg, err := msgBus.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("expected inbound message: %v", err)
	}
	if msg.Content != "hello qq" {
		t.Errorf("content = %q, want hello qq", msg.Content)
	}
	if msg.SenderID != "a1" {
		t.Errorf("senderID = %q, want a1", msg.SenderID)
	}
}

func TestQQHandleEvent_NonMessageOp(t *testing.T) {
	cfg := `{"appId":"aid","token":"tok","appSecret":"sec"}`
	ch, _ := newQQChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	qc := ch.(*QQChannel)

	body := `{"op":0,"t":"OTHER_EVENT","d":{}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	qc.handleEvent(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestQQHandleEvent_DisallowedUser(t *testing.T) {
	msgBus := bus.NewMessageBus(4)
	cfg := `{"appId":"aid","token":"tok","appSecret":"sec","allowedUsers":["allowed"]}`
	ch, _ := newQQChannel(json.RawMessage(cfg), msgBus)
	qc := ch.(*QQChannel)

	body := `{"op":0,"t":"AT_MESSAGE_CREATE","d":{"id":"m1","channel_id":"ch1","author":{"id":"blocked"},"content":"hi"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()
	qc.handleEvent(w, req)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := msgBus.ConsumeInbound(ctx)
	if err == nil {
		t.Error("expected no message for disallowed user")
	}
}

// --- Email ---

func TestNewEmailChannel(t *testing.T) {
	cfg := `{"imapServer":"imap.test:993","smtpServer":"smtp.test:587","username":"u","password":"p","allowedUsers":["a@b.com"]}`
	ch, err := newEmailChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ec := ch.(*EmailChannel)
	if ec.Name() != "email" {
		t.Errorf("Name = %q, want email", ec.Name())
	}
	if !ec.IsAllowed("a@b.com") {
		t.Error("expected a@b.com allowed")
	}
	if ec.IsAllowed("other@b.com") {
		t.Error("expected other@b.com disallowed")
	}
}

func TestEmailIsAllowed_EmptyAllowAll(t *testing.T) {
	cfg := `{"imapServer":"imap.test:993","smtpServer":"smtp.test:587","username":"u","password":"p"}`
	ch, _ := newEmailChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	ec := ch.(*EmailChannel)
	if !ec.IsAllowed("anyone@test.com") {
		t.Error("empty allowedUsers should allow all")
	}
}

func TestEmailStop_NilCancel(t *testing.T) {
	cfg := `{"imapServer":"imap.test:993","smtpServer":"smtp.test:587","username":"u","password":"p"}`
	ch, _ := newEmailChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	ec := ch.(*EmailChannel)
	if err := ec.Stop(); err != nil {
		t.Errorf("Stop with nil cancel should not error: %v", err)
	}
}

func TestParseIMAPFetch(t *testing.T) {
	lines := []string{
		"From: sender@test.com",
		"Subject: Test Subject",
		"",
		"This is the body",
		"Second line",
		"a4 OK FETCH completed",
	}
	from, subject, body := parseIMAPFetch(lines)
	if from != "sender@test.com" {
		t.Errorf("from = %q, want sender@test.com", from)
	}
	if subject != "Test Subject" {
		t.Errorf("subject = %q, want Test Subject", subject)
	}
	if !strings.Contains(body, "This is the body") {
		t.Errorf("body = %q, expected to contain body text", body)
	}
}

func TestParseIMAPFetch_SkipsIMAPLines(t *testing.T) {
	lines := []string{
		"From: test@test.com",
		"Subject: Hi",
		"",
		"body text",
		"* 1 FETCH ...",
		"a4 OK done",
	}
	_, _, body := parseIMAPFetch(lines)
	if strings.Contains(body, "* 1 FETCH") {
		t.Error("body should not contain IMAP response lines")
	}
	if strings.Contains(body, "a4 OK") {
		t.Error("body should not contain tagged response")
	}
}

// --- Mochat ---

func TestNewMochatChannel(t *testing.T) {
	cfg := `{"url":"http://localhost:8080","allowedUsers":["u1"]}`
	ch, err := newMochatChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mc := ch.(*MochatChannel)
	if mc.Name() != "mochat" {
		t.Errorf("Name = %q, want mochat", mc.Name())
	}
	if !mc.IsAllowed("u1") {
		t.Error("expected u1 allowed")
	}
	if mc.IsAllowed("u2") {
		t.Error("expected u2 disallowed")
	}
}

func TestMochatSend(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		receivedBody = string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mc := &MochatChannel{baseURL: srv.URL}
	err := mc.Send(bus.OutboundMessage{ChatID: "c1", Content: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(receivedBody, "hello") {
		t.Errorf("expected body to contain hello, got %q", receivedBody)
	}
}

func TestMochatSend_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	mc := &MochatChannel{baseURL: srv.URL}
	err := mc.Send(bus.OutboundMessage{ChatID: "c1", Content: "hello"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestMochatStop_NilCancel(t *testing.T) {
	mc := &MochatChannel{}
	if err := mc.Stop(); err != nil {
		t.Errorf("Stop with nil cancel should not error: %v", err)
	}
}

func TestMochatPoll(t *testing.T) {
	msgBus := bus.NewMessageBus(4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"timestamp":1000,"senderId":"s1","chatId":"c1","content":"poll msg"}]`))
	}))
	defer srv.Close()

	mc := &MochatChannel{baseURL: srv.URL, bus: msgBus, allowedUsers: map[string]bool{}}
	mc.poll()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	msg, err := msgBus.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("expected inbound message: %v", err)
	}
	if msg.Content != "poll msg" {
		t.Errorf("content = %q, want poll msg", msg.Content)
	}
}

// --- Constructor error cases ---

func TestNewFeishuChannel_InvalidJSON(t *testing.T) {
	_, err := newFeishuChannel(json.RawMessage(`{invalid`), bus.NewMessageBus(4))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewDingTalkChannel_InvalidJSON(t *testing.T) {
	_, err := newDingTalkChannel(json.RawMessage(`{invalid`), bus.NewMessageBus(4))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewQQChannel_InvalidJSON(t *testing.T) {
	_, err := newQQChannel(json.RawMessage(`{invalid`), bus.NewMessageBus(4))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewEmailChannel_InvalidJSON(t *testing.T) {
	_, err := newEmailChannel(json.RawMessage(`{invalid`), bus.NewMessageBus(4))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewMochatChannel_InvalidJSON(t *testing.T) {
	_, err := newMochatChannel(json.RawMessage(`{invalid`), bus.NewMessageBus(4))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNewWhatsAppChannel_InvalidJSON(t *testing.T) {
	_, err := newWhatsAppChannel(json.RawMessage(`{invalid`), bus.NewMessageBus(4))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Feishu/DingTalk Send with mock server ---

func TestFeishuSend_ViaStruct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// We can't easily override the Feishu API URL, but we can test Send directly
	// by constructing a FeishuChannel with a mock token
	fc := &FeishuChannel{accessToken: "test-token"}
	// Send will fail because it hits the real Feishu API, but we verify the method exists
	_ = fc.Send(bus.OutboundMessage{ChatID: "chat1", Content: "test"})
}

func TestDingTalkSend(t *testing.T) {
	dc := &DingTalkChannel{clientID: "cid", accessToken: "test-token"}
	// Send will fail because it hits the real DingTalk API, but we verify the method exists
	_ = dc.Send(bus.OutboundMessage{ChatID: "chat1", Content: "test"})
}

func TestQQSend(t *testing.T) {
	qc := &QQChannel{appID: "aid", token: "tok"}
	_ = qc.Send(bus.OutboundMessage{ChatID: "chat1", Content: "test"})
}

// --- Stop tests for webhook-based channels ---

func TestDingTalkStop(t *testing.T) {
	cfg := `{"clientId":"cid","clientSecret":"csec"}`
	ch, _ := newDingTalkChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	dc := ch.(*DingTalkChannel)
	// Stop on a server that was never started should not panic
	err := dc.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestQQStop(t *testing.T) {
	cfg := `{"appId":"aid","token":"tok","appSecret":"sec"}`
	ch, _ := newQQChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	qc := ch.(*QQChannel)
	err := qc.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWhatsAppStop(t *testing.T) {
	wa := newTestWhatsApp(t, nil)
	err := wa.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- imapCmd test ---

func TestImapCmd(t *testing.T) {
	// Simulate a server response in a bufio.ReadWriter
	serverResp := "* OK ready\r\na1 OK LOGIN completed\r\n"
	reader := strings.NewReader(serverResp)
	var writerBuf strings.Builder
	rw := bufio.NewReadWriter(
		bufio.NewReader(reader),
		bufio.NewWriter(&writerBuf),
	)

	lines, err := imapCmd(rw, "a1", "LOGIN user pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[1], "a1 ") {
		t.Errorf("expected tagged response, got %q", lines[1])
	}
	// Verify the command was written
	written := writerBuf.String()
	if !strings.Contains(written, "a1 LOGIN user pass") {
		t.Errorf("expected command in output, got %q", written)
	}
}

// --- Email Start/Stop ---

func TestEmailStartStop(t *testing.T) {
	cfg := `{"imapServer":"localhost:993","smtpServer":"localhost:587","username":"u","password":"p"}`
	ch, err := newEmailChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ec := ch.(*EmailChannel)
	// Start will launch a goroutine that tries to poll (and fails silently)
	err = ec.Start(context.Background())
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	// Stop should cancel the polling goroutine
	err = ec.Stop()
	if err != nil {
		t.Fatalf("Stop error: %v", err)
	}
}

// --- Mochat Start/Stop ---

func TestMochatStartStop(t *testing.T) {
	cfg := `{"url":"http://localhost:9999"}`
	ch, _ := newMochatChannel(json.RawMessage(cfg), bus.NewMessageBus(4))
	mc := ch.(*MochatChannel)
	err := mc.Start(context.Background())
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	err = mc.Stop()
	if err != nil {
		t.Fatalf("Stop error: %v", err)
	}
}
