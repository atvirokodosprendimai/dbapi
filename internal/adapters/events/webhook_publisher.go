package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

const defaultWebhookTimeout = 10 * time.Second

// WebhookPublisher sends outbox events to a configured HTTP endpoint.
// Each request is signed with HMAC-SHA256 so the receiver can verify authenticity.
// Non-2xx responses are treated as errors, letting the outbox dispatcher apply its
// built-in retry/dead-letter policy.
type WebhookPublisher struct {
	url    string
	secret []byte
	client *http.Client
}

// NewWebhookPublisher returns a WebhookPublisher that POSTs events to url and
// signs them with secret using HMAC-SHA256.  A zero or negative timeout falls
// back to defaultWebhookTimeout (10 s).
func NewWebhookPublisher(url, secret string, timeout time.Duration) *WebhookPublisher {
	if timeout <= 0 {
		timeout = defaultWebhookTimeout
	}
	return &WebhookPublisher{
		url:    url,
		secret: []byte(secret),
		client: &http.Client{Timeout: timeout},
	}
}

// Publish marshals event to JSON, signs the body, and POSTs it to the
// configured webhook URL.  The following headers are set on every request:
//
//	Content-Type:           application/json
//	X-Dbapi-Topic:          <topic>
//	X-Dbapi-Event-Type:     <event.EventType>
//	X-Dbapi-Tenant:         <event.TenantID>
//	X-Hub-Signature-256:    sha256=<hex-encoded HMAC-SHA256>
func (p *WebhookPublisher) Publish(ctx context.Context, topic string, event domain.EventEnvelope) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	sig := p.sign(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dbapi-Topic", topic)
	req.Header.Set("X-Dbapi-Event-Type", event.EventType)
	req.Header.Set("X-Dbapi-Tenant", event.TenantID)
	req.Header.Set("X-Hub-Signature-256", "sha256="+sig)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// sign returns the lowercase hex-encoded HMAC-SHA256 of payload using p.secret.
func (p *WebhookPublisher) sign(payload []byte) string {
	mac := hmac.New(sha256.New, p.secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
