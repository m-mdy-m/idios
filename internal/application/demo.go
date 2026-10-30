package application

import (
	"context"
	"idios/internal/domain"
	"idios/internal/ports"
)

type DemoService struct{ store ports.DemoStore }

func (s *DemoService) SeedDemo(ctx context.Context) (domain.SeedReport, error) {
	return s.store.SeedDemo(ctx)
}
