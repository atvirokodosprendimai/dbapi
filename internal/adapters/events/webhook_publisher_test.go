package events

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

func TestWebhookPublisherSuccess(t *testing.T) {
	var gotBody []byte
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	secret := "test-secret"
	pub := NewWebhookPublisher(srv.URL, secret, 5*time.Second)

	event := domain.EventEnvelope{
		EventID:       "evt-1",
		EventType:     "record.created",
		TenantID:      "tenant-a",
		AggregateType: "contacts",
		AggregateID:   "c1",
		SchemaVersion: 1,
	}

	if err := pub.Publish(context.Background(), "events.tenant-a.record.created", event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify headers
	if ct := gotHeaders.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if topic := gotHeaders.Get("X-Dbapi-Topic"); topic != "events.tenant-a.record.created" {
		t.Errorf("X-Dbapi-Topic = %q, want events.tenant-a.record.created", topic)
	}
	if et := gotHeaders.Get("X-Dbapi-Event-Type"); et != "record.created" {
		t.Errorf("X-Dbapi-Event-Type = %q, want record.created", et)
	}
	if ten := gotHeaders.Get("X-Dbapi-Tenant"); ten != "tenant-a" {
		t.Errorf("X-Dbapi-Tenant = %q, want tenant-a", ten)
	}

	// Verify HMAC-SHA256 signature
	sigHeader := gotHeaders.Get("X-Hub-Signature-256")
	if !strings.HasPrefix(sigHeader, "sha256=") {
		t.Fatalf("X-Hub-Signature-256 header missing or malformed: %q", sigHeader)
	}
	gotSig := strings.TrimPrefix(sigHeader, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(gotBody)
	wantSig := hex.EncodeToString(mac.Sum(nil))
	if gotSig != wantSig {
		t.Errorf("signature mismatch: got %q, want %q", gotSig, wantSig)
	}

	// Verify body contains the event
	var decoded domain.EventEnvelope
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if decoded.EventID != event.EventID {
		t.Errorf("EventID = %q, want %q", decoded.EventID, event.EventID)
	}
}

func TestWebhookPublisherNon2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	pub := NewWebhookPublisher(srv.URL, "secret", 5*time.Second)
	event := domain.EventEnvelope{EventID: "evt-2", EventType: "record.updated", SchemaVersion: 1}

	err := pub.Publish(context.Background(), "events.t.record.updated", event)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code 500, got: %v", err)
	}
}

func TestWebhookPublisherContextCancellation(t *testing.T) {
	// Server that hangs until closed
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	pub := NewWebhookPublisher(srv.URL, "secret", 5*time.Second)
	event := domain.EventEnvelope{EventID: "evt-3", EventType: "record.created", SchemaVersion: 1}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := pub.Publish(ctx, "events.t.record.created", event)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected error to wrap context.Canceled, got: %v", err)
	}
}

func TestWebhookPublisherZeroTimeoutUsesDefault(t *testing.T) {
	pub := NewWebhookPublisher("http://localhost:9", "s", 0)
	if pub.client.Timeout != defaultWebhookTimeout {
		t.Errorf("timeout = %v, want %v", pub.client.Timeout, defaultWebhookTimeout)
	}
}
