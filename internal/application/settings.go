package application

import (
	"context"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type SettingsService struct{ store ports.SettingsStore }

func (s *SettingsService) SetAppSetting(ctx context.Context, key, value string) error {
	if err := invalidInput(validation.Text("setting key", key, true, 128)); err != nil {
		return err
	}
	if err := invalidInput(validation.Text("setting value", value, false, 20000)); err != nil {
		return err
	}
	return s.store.SetAppSetting(ctx, strings.TrimSpace(key), value)
}

func (s *SettingsService) GetAppSetting(ctx context.Context, key string) (string, error) {
	return s.store.GetAppSetting(ctx, key)
}
