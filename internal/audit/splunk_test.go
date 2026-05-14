package audit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subzone/Agentctl/internal/ports"
)

func TestNewSplunkSinkRequiresEndpointAndToken(t *testing.T) {
	if _, err := NewSplunkSink("", "x", true); err == nil {
		t.Fatal("expected endpoint error")
	}
	if _, err := NewSplunkSink("http://example", "", true); err == nil {
		t.Fatal("expected token error")
	}
}

func TestSplunkSinkEmit(t *testing.T) {
	var gotAuth string
	var gotEvent ports.AuditEvent
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/services/collector" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var payload struct {
			Event ports.AuditEvent `json:"event"`
		}
		if err := json.Unmarshal(b, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		gotEvent = payload.Event
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"Success","code":0}`))
	}))
	defer ts.Close()

	sink, err := NewSplunkSink(ts.URL, "tok123", true)
	if err != nil {
		t.Fatal(err)
	}
	err = sink.Emit(context.Background(), ports.AuditEvent{Type: "session_start", SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Splunk tok123" {
		t.Fatalf("auth=%q", gotAuth)
	}
	if gotEvent.Type != "session_start" || gotEvent.SessionID != "s1" {
		t.Fatalf("event=%+v", gotEvent)
	}
}

func TestSplunkSinkEmitNon2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("bad token"))
	}))
	defer ts.Close()

	sink, err := NewSplunkSink(ts.URL, "tok", true)
	if err != nil {
		t.Fatal(err)
	}
	err = sink.Emit(context.Background(), ports.AuditEvent{Type: "x"})
	if err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("err=%v", err)
	}
}
