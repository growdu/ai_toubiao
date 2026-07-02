package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bidwriter/services/notify-svc/internal/model"
	"github.com/google/uuid"
)

// fakeSMTPDialer records the (addr, msg) for assertion and can be programmed
// to return an error.
type fakeSMTPDialer struct {
	calls   []smtpCall
	sendErr error
}

type smtpCall struct {
	addr string
	msg  []byte
}

func (f *fakeSMTPDialer) SendMail(_ context.Context, addr string, msg []byte) error {
	f.calls = append(f.calls, smtpCall{addr: addr, msg: append([]byte(nil), msg...)})
	return f.sendErr
}

// fakeTransport records each POST for assertion and can be programmed
// with a per-URL response (status code + body).
type fakeTransport struct {
	calls    []transportCall
	responder func(url string, body []byte) (int, []byte, error)
}

type transportCall struct {
	url     string
	body    []byte
	headers map[string]string
}

func (f *fakeTransport) Post(_ context.Context, url string, body []byte, headers map[string]string) (int, []byte, error) {
	f.calls = append(f.calls, transportCall{url: url, body: append([]byte(nil), body...), headers: headers})
	if f.responder != nil {
		return f.responder(url, body)
	}
	return 200, []byte(`{"ok":true}`), nil
}

func newTestNotifier(smtp SMTPDialer, web Transport) *Notifier {
	return NewNotifier(smtp, web, "noreply@example.com", "[bidwriter] ")
}

func reqEmail(addr, subject, body string) *model.SendRequest {
	return &model.SendRequest{
		Type:    model.TypeBidGenerated,
		Channel: model.ChannelEmail,
		UserID:  uuid.New(),
		Subject: subject,
		Body:    body,
		Address: addr,
	}
}

// ---- Email ----

func TestNotifier_Email_BuildsMIMEAndSends(t *testing.T) {
	dialer := &fakeSMTPDialer{}
	n := newTestNotifier(dialer, &fakeTransport{})
	if err :=	n.Email(context.Background(), reqEmail("alice@example.com", "Hello", "Body line 1\nLine 2")); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(dialer.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(dialer.calls))
	}
	got := dialer.calls[0]
	if got.addr != "alice@example.com" {
		t.Errorf("addr = %q, want alice@example.com", got.addr)
	}
	// Header sanity: From / To / Subject / MIME-Version / Content-Type all present.
	// Subject may be plain ASCII (RFC 2047 — no encoded-word) or RFC 2047
	// encoded; both are valid. Assert the ASCII case directly so we don't
	// over-specify the encoding choice.
	msgStr := string(got.msg)
	for _, want := range []string{
		"From: noreply@example.com",
		"To: alice@example.com",
		"Subject: [bidwriter] Hello",
		"MIME-Version: 1.0",
		"Content-Type: text/plain",
	} {
		if !strings.Contains(msgStr, want) {
			t.Errorf("email msg missing %q\n--- msg ---\n%s", want, msgStr)
		}
	}
	// Body lines embedded verbatim.
	if !strings.Contains(msgStr, "Body line 1") || !strings.Contains(msgStr, "Line 2") {
		t.Errorf("body content not embedded\n%s", msgStr)
	}
}

func TestNotifier_Email_UnicodeSubject_EncodesAsRFC2047(t *testing.T) {
	dialer := &fakeSMTPDialer{}
	n := newTestNotifier(dialer, &fakeTransport{})
	err := n.Email(context.Background(), reqEmail("alice@example.com", "标书生成完成", "Body"))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	msgStr := string(dialer.calls[0].msg)
	// Non-ASCII subject MUST be RFC 2047 encoded (the =?UTF-8?B?...?= form).
	if !strings.Contains(msgStr, "Subject: =?UTF-8?") {
		t.Errorf("expected RFC 2047 encoded Subject, got\n%s", msgStr)
	}
	if !strings.Contains(msgStr, "Body") {
		t.Errorf("body missing\n%s", msgStr)
	}
}

func TestNotifier_Email_SMTPSendError_Propagates(t *testing.T) {
	dialer := &fakeSMTPDialer{sendErr: errors.New("smtp 421 try later")}
	n := newTestNotifier(dialer, &fakeTransport{})
	err :=	n.Email(context.Background(), reqEmail("a@b.com", "S", "B"))
	if err == nil || !strings.Contains(err.Error(), "smtp 421") {
		t.Errorf("err = %v, want smtp 421 wrapped", err)
	}
}

func TestNotifier_Email_MissingAddress_ReturnsError(t *testing.T) {
	dialer := &fakeSMTPDialer{}
	n := newTestNotifier(dialer, &fakeTransport{})
	err :=	n.Email(context.Background(), reqEmail("", "S", "B"))
	if err == nil {
		t.Fatal("expected error for empty address")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Errorf("err = %v, want one mentioning address", err)
	}
	if len(dialer.calls) != 0 {
		t.Errorf("smtp should not be called with empty addr, got %d calls", len(dialer.calls))
	}
}

// ---- DingTalk ----

func TestNotifier_DingTalk_PostsTextJSON(t *testing.T) {
	web := &fakeTransport{}
	n := newTestNotifier(&fakeSMTPDialer{}, web)
	webhook := "https://oapi.dingtalk.com/robot/send?access_token=TOK"
	err :=	n.DingTalk(context.Background(), &model.SendRequest{
		Type:    model.TypeBidGenerated,
		Channel: model.ChannelDingTalk,
		UserID:  uuid.New(),
		Subject: "标题",
		Body:    "正文",
		Address: webhook,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(web.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(web.calls))
	}
	got := web.calls[0]
	if got.url != webhook {
		t.Errorf("url = %q, want %q", got.url, webhook)
	}
	if ctype := got.headers["Content-Type"]; ctype != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ctype)
	}
	// Body must contain msgtype:text + content with both subject and body.
	s := string(got.body)
	for _, want := range []string{`"msgtype":"text"`, `"content":"标题\n正文"`} {
		if !strings.Contains(s, want) {
			t.Errorf("dingtalk payload missing %s\n--- payload ---\n%s", want, s)
		}
	}
}

func TestNotifier_DingTalk_NonOKStatus_ReturnsError(t *testing.T) {
	web := &fakeTransport{
		responder: func(_ string, _ []byte) (int, []byte, error) {
			return 500, []byte(`{"errcode":-1}`), nil
		},
	}
	n := newTestNotifier(&fakeSMTPDialer{}, web)
	err :=	n.DingTalk(context.Background(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.ChannelDingTalk,
		UserID: uuid.New(), Subject: "S", Body: "B",
		Address: "https://oapi.dingtalk.com/robot/send?access_token=x",
	})
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want one mentioning 500", err)
	}
}

func TestNotifier_DingTalk_TransportError_Propagates(t *testing.T) {
	web := &fakeTransport{
		responder: func(_ string, _ []byte) (int, []byte, error) {
			return 0, nil, errors.New("connection refused")
		},
	}
	n := newTestNotifier(&fakeSMTPDialer{}, web)
	err :=	n.DingTalk(context.Background(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.ChannelDingTalk,
		UserID: uuid.New(), Subject: "S", Body: "B",
		Address: "https://oapi.dingtalk.com/robot/send?access_token=x",
	})
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("err = %v, want connection refused", err)
	}
}

// ---- WeCom ----

func TestNotifier_WeCom_PostsTextJSON(t *testing.T) {
	web := &fakeTransport{
		responder: func(_ string, _ []byte) (int, []byte, error) {
			return 200, []byte(`{"errcode":0,"errmsg":"ok"}`), nil
		},
	}
	n := newTestNotifier(&fakeSMTPDialer{}, web)
	err :=	n.WeCom(context.Background(), &model.SendRequest{
		Type:    model.TypeAuditCompleted,
		Channel: model.ChannelWeCom,
		UserID:  uuid.New(),
		Subject: "审计完成",
		Body:    "通过",
		Address: "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=TOK",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(web.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(web.calls))
	}
	got := web.calls[0]
	if !strings.Contains(got.url, "qyapi.weixin.qq.com") {
		t.Errorf("url = %q, want wecom domain", got.url)
	}
	s := string(got.body)
	if !strings.Contains(s, `"msgtype":"text"`) || !strings.Contains(s, `"content":"审计完成\n通过"`) {
		t.Errorf("wecom payload wrong\n%s", s)
	}
}

func TestNotifier_WeCom_ErrCodeNonZero_ReturnsError(t *testing.T) {
	web := &fakeTransport{
		responder: func(_ string, _ []byte) (int, []byte, error) {
			return 200, []byte(`{"errcode":40001,"errmsg":"invalid credential"}`), nil
		},
	}
	n := newTestNotifier(&fakeSMTPDialer{}, web)
	err :=	n.WeCom(context.Background(), &model.SendRequest{
		Type: model.TypeAuditCompleted, Channel: model.ChannelWeCom,
		UserID: uuid.New(), Subject: "S", Body: "B",
		Address: "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=x",
	})
	if err == nil || !strings.Contains(err.Error(), "40001") {
		t.Errorf("err = %v, want one mentioning 40001", err)
	}
}

func TestNotifier_WeCom_TransportError_Propagates(t *testing.T) {
	web := &fakeTransport{
		responder: func(_ string, _ []byte) (int, []byte, error) {
			return 0, nil, errors.New("tls handshake")
		},
	}
	n := newTestNotifier(&fakeSMTPDialer{}, web)
	err :=	n.WeCom(context.Background(), &model.SendRequest{
		Type: model.TypeAuditCompleted, Channel: model.ChannelWeCom,
		UserID: uuid.New(), Subject: "S", Body: "B",
		Address: "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=x",
	})
	if err == nil || !strings.Contains(err.Error(), "tls handshake") {
		t.Errorf("err = %v, want tls handshake", err)
	}
}
