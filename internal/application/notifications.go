package application

import (
	"context"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type NotificationsService struct{ store ports.NotificationStore }

func (s *NotificationsService) ListNotifications(ctx context.Context, limit int) ([]domain.Notification, error) {
	return s.store.ListNotifications(ctx, limit)
}

func (s *NotificationsService) CreateNotification(ctx context.Context, title, body, level string) (domain.Notification, error) {
	if err := invalidInput(validation.Text("notification title", title, true, 200)); err != nil {
		return domain.Notification{}, err
	}
	if err := invalidInput(validation.Text("notification body", body, false, 10000)); err != nil {
		return domain.Notification{}, err
	}
	if err := invalidInput(validation.Text("notification level", level, true, 32)); err != nil {
		return domain.Notification{}, err
	}
	if err := invalidInput(validation.OneOf("notification level", level, "info", "warning", "error")); err != nil {
		return domain.Notification{}, err
	}
	return s.store.CreateNotification(ctx, strings.TrimSpace(title), body, strings.TrimSpace(level))
}

func (s *NotificationsService) DeleteNotification(ctx context.Context, id string) error {
	if err := invalidInput(validation.Text("notification reference", id, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteNotification(ctx, strings.TrimSpace(id))
}

func (s *NotificationsService) MarkNotificationsRead(ctx context.Context) error {
	return s.store.MarkNotificationsRead(ctx)
}
