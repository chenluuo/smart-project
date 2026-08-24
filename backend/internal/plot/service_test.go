package plot

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

type plotStoreStub struct {
	plots         []Plot
	plot          *Plot
	listErr       error
	getErr        error
	updateCropErr error
	ownerID       uint64
	plotID        uint64
	cropType      string
	plantingTime  time.Time
	listCalls     int
}

func (s *plotStoreStub) FindByOwner(_ context.Context, ownerID uint64) ([]Plot, error) {
	s.listCalls++
	s.ownerID = ownerID
	return s.plots, s.listErr
}

func (s *plotStoreStub) FindByIDAndOwner(_ context.Context, plotID, ownerID uint64) (*Plot, error) {
	s.plotID = plotID
	s.ownerID = ownerID
	return s.plot, s.getErr
}

func (s *plotStoreStub) UpdateCrop(_ context.Context, plotID, ownerID uint64, cropType string, plantingTime time.Time) error {
	s.plotID = plotID
	s.ownerID = ownerID
	s.cropType = cropType
	s.plantingTime = plantingTime
	return s.updateCropErr
}

func TestServiceScopesPlotsToOwner(t *testing.T) {
	store := &plotStoreStub{plots: []Plot{{ID: 11, OwnerID: 7, Code: "A1"}}, plot: &Plot{ID: 11, OwnerID: 7, Code: "A1"}}
	service := NewService(store)

	results, err := service.List(context.Background(), 7)
	if err != nil || len(results) != 1 || store.ownerID != 7 {
		t.Fatalf("List() = (%+v, %v), ownerID = %d", results, err, store.ownerID)
	}
	result, err := service.Get(context.Background(), 7, 11)
	if err != nil || result.ID != 11 || store.ownerID != 7 || store.plotID != 11 {
		t.Fatalf("Get() = (%+v, %v), ownerID = %d, plotID = %d", result, err, store.ownerID, store.plotID)
	}
}

func TestServiceMapsMissingPlot(t *testing.T) {
	service := NewService(&plotStoreStub{getErr: gorm.ErrRecordNotFound})
	if _, err := service.Get(context.Background(), 7, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestServiceWrapsStoreFailures(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	service := NewService(&plotStoreStub{listErr: databaseErr, getErr: databaseErr})
	if _, err := service.List(context.Background(), 7); !errors.Is(err, databaseErr) {
		t.Fatalf("List() error = %v, want wrapped database error", err)
	}
	if _, err := service.Get(context.Background(), 7, 11); !errors.Is(err, databaseErr) {
		t.Fatalf("Get() error = %v, want wrapped database error", err)
	}
}

func TestServiceUpdateCropSetsCropNameAndPlantingTime(t *testing.T) {
	store := &plotStoreStub{}
	service := NewService(store)

	result, err := service.UpdateCrop(context.Background(), 7, 11, "  番茄  ")
	if err != nil {
		t.Fatalf("UpdateCrop() error = %v", err)
	}
	if store.ownerID != 7 || store.plotID != 11 {
		t.Fatalf("UpdateCrop() ownerID = %d, plotID = %d", store.ownerID, store.plotID)
	}
	if store.cropType != "番茄" {
		t.Fatalf("UpdateCrop() cropType = %q, want trimmed %q", store.cropType, "番茄")
	}
	if store.plantingTime.IsZero() {
		t.Fatalf("UpdateCrop() plantingTime is zero")
	}
	if result == nil || result.ID != 11 || result.CropType == nil || *result.CropType != "番茄" || result.PlantingTime == nil {
		t.Fatalf("UpdateCrop() result = %+v", result)
	}
}

func TestServiceUpdateCropRejectsInvalidName(t *testing.T) {
	longName := strings.Repeat("菜", 65)
	for _, cropName := range []string{"", "   ", longName} {
		store := &plotStoreStub{}
		service := NewService(store)
		if _, err := service.UpdateCrop(context.Background(), 7, 11, cropName); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("UpdateCrop(%q) error = %v, want ErrInvalidInput", cropName, err)
		}
		if store.cropType != "" {
			t.Fatalf("UpdateCrop(%q) reached store, cropType = %q", cropName, store.cropType)
		}
	}
}

func TestServiceUpdateCropMapsMissingPlot(t *testing.T) {
	service := NewService(&plotStoreStub{updateCropErr: gorm.ErrRecordNotFound})
	if _, err := service.UpdateCrop(context.Background(), 7, 99, "番茄"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateCrop() error = %v, want ErrNotFound", err)
	}
}

func TestServiceUpdateCropWrapsStoreFailure(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	service := NewService(&plotStoreStub{updateCropErr: databaseErr})
	if _, err := service.UpdateCrop(context.Background(), 7, 11, "番茄"); !errors.Is(err, databaseErr) {
		t.Fatalf("UpdateCrop() error = %v, want wrapped database error", err)
	}
}
