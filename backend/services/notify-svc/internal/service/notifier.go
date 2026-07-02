package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/bidwriter/services/notify-svc/internal/model"
)

// SMTPDialer is the seam for SMTP delivery. Production uses *netSMTPDialer
// (wrapping net/smtp + crypto/tls); tests inject *fakeSMTPDialer to assert
// the message body without a live server.
type SMTPDialer interface {
	SendMail(ctx context.Context, addr string, msg []byte) error
}

// Transport is the seam for HTTP-based webhooks (DingTalk, WeCom, ...).
type Transport interface {
	Post(ctx context.Context, url string, body []byte, headers map[string]string) (status int, respBody []byte, err error)
}

// SMTPConfig is the operator-supplied credentials used by the real dialer.
// Empty Username/Password mean "anonymous relay", which is what most local
// dev / docker-compose mailhog setups expect.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Notifier dispatches a SendRequest to the right transport. It is the
// production implementation behind NotifyService.Send; the test seam lives
// in the dispatch table (`channelSenders`) so service / handler-level
// tests don't have to set up transports.
type Notifier struct {
	smtpDialer    SMTPDialer
	webhookClient Transport

	// From address & subject prefix used by Email. Configurable so tests
	// can build deterministic messages.
	from          string
	subjectPrefix string
}

// NewNotifier wires the given transports into a Notifier. It is the only
// place transport choice happens, so production / tests diverge here.
func NewNotifier(smtpDialer SMTPDialer, webhookClient Transport, from, subjectPrefix string) *Notifier {
	if from == "" {
		from = "noreply@bidwriter.local"
	}
	return &Notifier{
		smtpDialer:    smtpDialer,
		webhookClient: webhookClient,
		from:          from,
		subjectPrefix: subjectPrefix,
	}
}

// ---- Email ----

// Email dispatches a SendRequest over SMTP. The recipient is taken from
// req.Address (a preference.Address for email). The method returns an
// error when the address is empty so callers don't accidentally send
// "to: nobody" into the relay.
func (n *Notifier) Email(ctx context.Context, req *model.SendRequest) error {
	if strings.TrimSpace(req.Address) == "" {
		return errors.New("email: recipient address is required")
	}
	subject := n.subjectPrefix + req.Subject
	msg := buildMIMETextEmail(n.from, req.Address, subject, req.Body)
	return n.smtpDialer.SendMail(ctx, req.Address, msg)
}

// buildMIMETextEmail constructs an RFC 5322 email with a UTF-8 encoded
// subject (RFC 2047) and a text/plain body. The body is plain text — the
// audit / bid events produce plain text, not HTML.
func buildMIMETextEmail(from, to, subject, body string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(&b, "Content-Transfer-Encoding: 8bit\r\n")
	fmt.Fprintf(&b, "\r\n")
	b.WriteString(body)
	return b.Bytes()
}

// ---- DingTalk ----

// DingTalk posts a text message to a DingTalk custom robot webhook. The
// webhook URL (with access_token) is the address stored on the
// preference row. We use msgtype=text (not markdown) so unicode
// characters are preserved verbatim.
//
// DingTalk returns a JSON body with errcode=0 on success; non-2xx HTTP
// is treated as a transport error regardless of body content.
func (n *Notifier) DingTalk(ctx context.Context, req *model.SendRequest) error {
	if strings.TrimSpace(req.Address) == "" {
		return errors.New("dingtalk: webhook URL is required")
	}
	payload, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": req.Subject + "\n" + req.Body},
	})
	if err != nil {
		return fmt.Errorf("dingtalk: marshal payload: %w", err)
	}
	status, _, err := n.webhookClient.Post(ctx, req.Address, payload, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return fmt.Errorf("dingtalk: post: %w", err)
	}
	if status >= 400 {
		return fmt.Errorf("dingtalk: webhook returned HTTP %d", status)
	}
	return nil
}

// ---- WeCom ----

// WeCom posts a text message to a WeCom (企业微信) application webhook.
// WeCom returns HTTP 200 even for business errors, so we additionally
// parse errcode from the body and treat non-zero as failure. This is
// important because a stale access_token would otherwise look like
// success and the user would wonder why nothing arrived.
func (n *Notifier) WeCom(ctx context.Context, req *model.SendRequest) error {
	if strings.TrimSpace(req.Address) == "" {
		return errors.New("wecom: webhook URL is required")
	}
	payload, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": req.Subject + "\n" + req.Body},
	})
	if err != nil {
		return fmt.Errorf("wecom: marshal payload: %w", err)
	}
	status, respBody, err := n.webhookClient.Post(ctx, req.Address, payload, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return fmt.Errorf("wecom: post: %w", err)
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(respBody, &resp)
	if status >= 400 {
		return fmt.Errorf("wecom: webhook returned HTTP %d", status)
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("wecom: errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

// ---- Real transports (production) ----

// netSMTPDialer sends mail via net/smtp, optionally over implicit TLS
// (typically port 465) or STARTTLS (typically 587).
type netSMTPDialer struct {
	cfg       SMTPConfig
	tlsConfig *tls.Config
}

// NewSMTPDialer builds the production SMTP dialer from cfg. Returns nil
// when cfg.Host is empty so callers can detect the "not configured" case
// without checking strings.
func NewSMTPDialer(cfg SMTPConfig) SMTPDialer {
	if cfg.Host == "" {
		return nil
	}
	return &netSMTPDialer{
		cfg:       cfg,
		tlsConfig: &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12},
	}
}

// SendMail connects, optionally upgrades via STARTTLS, authenticates if
// credentials are set, and emits the message. The implementation is
// deliberately verbose — SMTP error messages are notoriously unhelpful,
// so each step is wrapped with the operation name to make debugging
// easier when a relay provider rejects the connection.
func (d *netSMTPDialer) SendMail(ctx context.Context, addr string, msg []byte) error {
	if d == nil {
		return errors.New("smtp: not configured")
	}
	host := d.cfg.Host
	port := d.cfg.Port
	if port == 0 {
		port = 587
	}
	addr_ := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr_, d.tlsConfig)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr_)
	}
	if err != nil {
		return fmt.Errorf("smtp: dial %s: %w", addr_, err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp: client: %w", err)
	}
	defer c.Close()

	if port != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(d.tlsConfig); err != nil {
				return fmt.Errorf("smtp: starttls: %w", err)
			}
		}
	}
	if d.cfg.Username != "" {
		auth := smtp.PlainAuth("", d.cfg.Username, d.cfg.Password, host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}
	if err := c.Mail(d.cfg.From); err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	if err := c.Rcpt(addr); err != nil {
		return fmt.Errorf("smtp: RCPT TO %q: %w", addr, err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: close body: %w", err)
	}
	return c.Quit()
}

// httpTransport is the production Transport — *http.Client with a
// sensible timeout. Webhooks are third-party calls; a wedged upstream
// shouldn't block a request indefinitely, hence the hard deadline.
type httpTransport struct {
	client *http.Client
}

// NewHTTPTransport returns a Transport backed by a fresh http.Client.
// A timeout of 0 falls back to 10s — most webhook providers respond in
// well under 2s, so anything longer is a strong signal of trouble.
func NewHTTPTransport(timeout time.Duration) Transport {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &httpTransport{client: &http.Client{Timeout: timeout}}
}

// Post sends a POST with the given body and headers. Only transport-level
// errors return a non-nil err; HTTP non-2xx is reported through the
// status return — callers must check it themselves because a webhook
// reporting errcode in the body (WeCom) is not a transport failure.
func (t *httpTransport) Post(ctx context.Context, url string, body []byte, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("http: build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("http: do: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("http: read body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}
