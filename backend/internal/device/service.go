package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

var (
	ErrNotFound           = errors.New("device not found")
	ErrAlreadyBound       = errors.New("device is already bound")
	ErrDeviceTypeMismatch = errors.New("device type does not match registered device")
	ErrInvalidInput       = errors.New("invalid device input")
)

type ListFilter struct {
	PlotID     *uint64
	Status     *Status
	DeviceType string
	Page       int
	PageSize   int
}

type ListItem struct {
	Device Device
	PlotID uint64
}

type ListResult struct {
	Items    []ListItem
	Page     int
	PageSize int
	Total    int64
}

type BindInput struct {
	SerialNo   string
	PlotID     uint64
	Name       string
	DeviceType string
}

type Store interface {
	ListByOwner(context.Context, uint64, ListFilter) ([]ListItem, int64, error)
	Bind(context.Context, uint64, BindInput) (*Device, error)
	Unbind(context.Context, uint64, uint64) error
	FindByIDAndOwner(context.Context, uint64, uint64) (*Device, error)
}

type Service struct{ devices Store }

func NewService(devices Store) *Service { return &Service{devices: devices} }

func (s *Service) List(ctx context.Context, ownerID uint64, filter ListFilter) (ListResult, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 || filter.PlotID != nil && *filter.PlotID == 0 || filter.Status != nil && !ValidStatus(*filter.Status) {
		return ListResult{}, ErrInvalidInput
	}
	filter.DeviceType = strings.TrimSpace(filter.DeviceType)
	if utf8.RuneCountInString(filter.DeviceType) > 64 {
		return ListResult{}, ErrInvalidInput
	}

	items, total, err := s.devices.ListByOwner(ctx, ownerID, filter)
	if err != nil {
		return ListResult{}, fmt.Errorf("list devices: %w", err)
	}
	if items == nil {
		items = []ListItem{}
	}
	return ListResult{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total}, nil
}

func (s *Service) Bind(ctx context.Context, ownerID uint64, input BindInput) (*Device, error) {
	input.SerialNo = strings.TrimSpace(input.SerialNo)
	input.Name = strings.TrimSpace(input.Name)
	input.DeviceType = strings.TrimSpace(input.DeviceType)
	if ownerID == 0 || input.PlotID == 0 || input.SerialNo == "" || input.Name == "" || input.DeviceType == "" ||
		utf8.RuneCountInString(input.SerialNo) > 128 || utf8.RuneCountInString(input.Name) > 128 || utf8.RuneCountInString(input.DeviceType) > 64 {
		return nil, ErrInvalidInput
	}
	result, err := s.devices.Bind(ctx, ownerID, input)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bind device: %w", err)
	}
	return result, nil
}

func (s *Service) Unbind(ctx context.Context, ownerID, deviceID uint64) error {
	if ownerID == 0 || deviceID == 0 {
		return ErrInvalidInput
	}
	if err := s.devices.Unbind(ctx, ownerID, deviceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("unbind device: %w", err)
	}
	return nil
}

func (s *Service) Status(ctx context.Context, ownerID, deviceID uint64) (*Device, error) {
	if ownerID == 0 || deviceID == 0 {
		return nil, ErrInvalidInput
	}
	result, err := s.devices.FindByIDAndOwner(ctx, deviceID, ownerID)
	if errors.Is(err, gorm.ErrRecordNotFound) || result == nil {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device status: %w", err)
	}
	return result, nil
}

func ValidStatus(status Status) bool {
	switch status {
	case StatusUnactivated, StatusOnline, StatusOffline, StatusReconnecting, StatusFault, StatusDisabled:
		return true
	default:
		return false
	}
}
