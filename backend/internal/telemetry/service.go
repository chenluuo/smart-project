package telemetry

import "context"

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) LatestByPlot(ctx context.Context, plotID uint64) (*Latest, error) {
	return s.store.LatestByPlot(ctx, plotID)
}
