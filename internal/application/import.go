package application

import (
	"context"
	"idios/internal/ports"
)

type ImportService struct{ store ports.ImportStore }

func (s *ImportService) ImportLegacy(ctx context.Context, path string) (ports.LegacyImportResult, error) {
	return s.store.ImportLegacy(ctx, path)
}

// GraphLayout returns the workspace's durable graph presentation settings.
