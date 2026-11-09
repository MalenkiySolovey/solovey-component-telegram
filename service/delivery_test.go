//go:build !minimal

package telegram_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	telegram "github.com/MalenkiySolovey/solovey-component-telegram/service"
)

type deliverySettings struct {
	enabled     bool
	token, chat string
}

func (s deliverySettings) GetTelegramEnabled() (bool, error)                   { return s.enabled, nil }
func (s deliverySettings) GetTelegramBotToken(context.Context) (string, error) { return s.token, nil }
func (s deliverySettings) GetTelegramChatID() (string, error)                  { return s.chat, nil }

func TestDeliveryAdmissionAndCredentialGates(t *testing.T) {
	for _, test := range []struct {
		name     string
		settings telegram.Settings
		lease    bool
		class    string
	}{
		{"missing admission", deliverySettings{true, "123456:synthetic-token", "42"}, false, "maintenance"},
		{"missing settings", nil, true, "settings"},
		{"disabled", deliverySettings{}, true, "disabled"},
		{"missing token", deliverySettings{enabled: true, chat: "42"}, true, "missing_token"},
		{"missing chat", deliverySettings{true, "123456:synthetic-token", ""}, true, "missing_chat"},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			svc := &telegram.Service{Settings: test.settings, Client: &http.Client{Transport: discoveryRoundTrip(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("must not send") })}}
			if test.lease {
				svc.Operations = fixtureOperation
			}
			result := svc.SendContext(t.Context(), "message")
			if result.Success || result.ErrorClass != test.class || calls != 0 {
				t.Fatalf("gate result=%+v calls=%d", result, calls)
			}
		})
	}
}

func TestDeliveryRetainsOperationLeaseUntilTransportDrains(t *testing.T) {
	entered, releaseHTTP := make(chan struct{}), make(chan struct{})
	var released atomic.Bool
	svc := &telegram.Service{Settings: deliverySettings{true, "123456:synthetic-token", "42"},
		Operations: func(ctx context.Context) (context.Context, func(), error) {
			return ctx, func() { released.Store(true) }, nil
		},
		Client: &http.Client{Transport: discoveryRoundTrip(func(r *http.Request) (*http.Response, error) {
			close(entered)
			select {
			case <-releaseHTTP:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			return discoveryResponse(200, `{"ok":true}`), nil
		})}}
	result := make(chan telegram.Result, 1)
	go func() { result <- svc.SendContext(t.Context(), "leased message") }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("transport did not start")
	}
	if released.Load() {
		t.Fatal("operation lease released during transport")
	}
	close(releaseHTTP)
	select {
	case got := <-result:
		if !got.Success || !released.Load() {
			t.Fatalf("send did not drain: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("send did not complete")
	}
}

func TestDeliveryMalformedLeaseFailsClosedAndReleasesPartialAdmission(t *testing.T) {
	released := false
	svc := &telegram.Service{Settings: deliverySettings{true, "123456:synthetic-token", "42"}, Operations: func(context.Context) (context.Context, func(), error) { return nil, func() { released = true }, nil }}
	if result := svc.SendContext(t.Context(), "message"); result.ErrorClass != "maintenance" || !released {
		t.Fatalf("malformed lease accepted: %+v", result)
	}
}

func TestDeliveryResponseClassificationAndRetryBound(t *testing.T) {
	svc := &telegram.Service{Settings: deliverySettings{true, "123456:synthetic-token", "42"}, Operations: fixtureOperation, Client: &http.Client{Transport: discoveryRoundTrip(func(r *http.Request) (*http.Response, error) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "private-value") {
			t.Fatal("secret was not redacted")
		}
		return discoveryResponse(429, `{"ok":false,"error_code":429,"parameters":{"retry_after":999999}}`), nil
	})}}
	result := svc.Send("Authorization: Bearer private-value")
	if result.Success || result.ErrorClass != "rate_limited" || result.RetryAfter != 300*time.Second {
		t.Fatalf("retry bound changed: %+v", result)
	}
}

type failedDocumentReader struct{}

func (failedDocumentReader) Read([]byte) (int, error) { return 0, errors.New("fixture source failed") }
func TestFailedDocumentStreamCannotReportSuccessfulDelivery(t *testing.T) {
	svc := &telegram.Service{Settings: deliverySettings{true, "123456:synthetic-token", "42"}, Operations: fixtureOperation, Client: &http.Client{Transport: discoveryRoundTrip(func(r *http.Request) (*http.Response, error) {
		_, _ = io.Copy(io.Discard, r.Body)
		return discoveryResponse(200, `{"ok":true}`), nil
	})}}
	if result := svc.SendDocumentStream(t.Context(), "fixture.aes", failedDocumentReader{}, "caption"); result.Success || result.ErrorClass != "payload" {
		t.Fatalf("failed upload reported success: %+v", result)
	}
}
