//go:build !minimal

package telegram

import (
	"context"
	"errors"
)

type telegramBotCredentials struct {
	Token  string
	ChatID string
}

func (s *Service) telegramEnabled() (bool, error) {
	if s == nil || s.Settings == nil {
		return false, errors.New("telegram settings are unavailable")
	}
	return s.Settings.GetTelegramEnabled()
}

func (s *Service) telegramBotCredentials(ctx context.Context) (telegramBotCredentials, Result) {
	enabled, err := s.telegramEnabled()
	if err != nil {
		return telegramBotCredentials{}, Result{ErrorClass: "settings"}
	}
	if !enabled {
		return telegramBotCredentials{}, Result{ErrorClass: "disabled"}
	}
	token, err := s.Settings.GetTelegramBotToken(ctx)
	if err != nil {
		return telegramBotCredentials{}, Result{ErrorClass: "settings"}
	}
	if token == "" {
		return telegramBotCredentials{}, Result{ErrorClass: "missing_token"}
	}
	chatID, err := s.Settings.GetTelegramChatID()
	if err != nil {
		return telegramBotCredentials{}, Result{ErrorClass: "settings"}
	}
	if chatID == "" {
		return telegramBotCredentials{}, Result{ErrorClass: "missing_chat"}
	}
	return telegramBotCredentials{Token: token, ChatID: chatID}, Result{Success: true}
}
