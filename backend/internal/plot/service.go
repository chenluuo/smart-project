package plot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

var (
	ErrNotFound     = errors.New("plot not found")
	ErrInvalidInput = errors.New("invalid plot input")
)

type Store interface {
	FindByOwner(ctx context.Context, ownerID uint64) ([]Plot, error)
	FindByIDAndOwner(ctx context.Context, plotID, ownerID uint64) (*Plot, error)
	UpdateCrop(ctx context.Context, plotID, ownerID uint64, cropType string, plantingTime time.Time) error
}

type Service struct {
	plots Store
}

func NewService(plots Store) *Service {
	return &Service{plots: plots}
}

func (s *Service) List(ctx context.Context, ownerID uint64) ([]Plot, error) {
	plots, err := s.plots.FindByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list plots: %w", err)
	}
	return plots, nil
}

func (s *Service) Get(ctx context.Context, ownerID, plotID uint64) (*Plot, error) {
	result, err := s.plots.FindByIDAndOwner(ctx, plotID, ownerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get plot: %w", err)
	}
	if result == nil {
		return nil, ErrNotFound
	}
	return result, nil
}

func (s *Service) UpdateCrop(ctx context.Context, ownerID, plotID uint64, cropName string) (*Plot, error) {
	cropName = strings.TrimSpace(cropName)
	if cropName == "" || utf8.RuneCountInString(cropName) > 64 {
		return nil, ErrInvalidInput
	}

	plantingTime := time.Now()
	if err := s.plots.UpdateCrop(ctx, plotID, ownerID, cropName, plantingTime); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update plot crop: %w", err)
	}
	return &Plot{ID: plotID, CropType: &cropName, PlantingTime: &plantingTime}, nil
}
