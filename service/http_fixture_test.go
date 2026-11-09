//go:build !minimal

package telegram_test

import (
	"context"
	"net/http"
	"net/url"
)

type testTelegramSettings struct{}

func (testTelegramSettings) GetTelegramEnabled() (bool, error) { return true, nil }
func (testTelegramSettings) GetTelegramBotToken(context.Context) (string, error) {
	return "123456:synthetic-token", nil
}
func (testTelegramSettings) GetTelegramChatID() (string, error) { return "42", nil }
func fixtureOperation(ctx context.Context) (context.Context, func(), error) {
	return ctx, func() {}, nil
}

type telegramServerRoundTripper struct {
	base      *url.URL
	transport http.RoundTripper
}

func (r telegramServerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = r.base.Scheme
	cloned.URL.Host = r.base.Host
	return r.transport.RoundTrip(cloned)
}
