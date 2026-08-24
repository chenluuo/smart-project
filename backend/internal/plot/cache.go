package plot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/cachex"
)

type cachedStore struct {
	next  Store
	cache cachex.JSONStore
	ttl   time.Duration
}

type cachedPlotList struct {
	SchemaVersion int    `json:"schemaVersion"`
	Items         []Plot `json:"items"`
}

type cachedPlotDetail struct {
	SchemaVersion int  `json:"schemaVersion"`
	Item          Plot `json:"item"`
}

func NewCachedStore(next Store, cache cachex.JSONStore, ttl time.Duration) Store {
	if cache == nil {
		return next
	}
	return &cachedStore{next: next, cache: cache, ttl: ttl}
}

func (s *cachedStore) FindByOwner(ctx context.Context, ownerID uint64) ([]Plot, error) {
	version, err := s.cache.Version(ctx, plotVersionKey(ownerID))
	if err != nil {
		slog.Warn("read plot cache version", "ownerId", ownerID, "error", err)
		return s.next.FindByOwner(ctx, ownerID)
	}
	key := fmt.Sprintf("agri:v1:cache:plots:owner:%d:g:%d:list", ownerID, version)
	var cached cachedPlotList
	if hit, cacheErr := s.cache.GetJSON(ctx, key, &cached); cacheErr == nil && hit && cached.SchemaVersion == cachex.SchemaVersion {
		return cached.Items, nil
	} else if cacheErr != nil {
		slog.Warn("read plot list cache", "key", key, "error", cacheErr)
	}
	result, err := s.next.FindByOwner(ctx, ownerID)
	if err == nil {
		if cacheErr := s.cache.SetJSON(ctx, key, cachedPlotList{SchemaVersion: cachex.SchemaVersion, Items: result}, s.ttl); cacheErr != nil {
			slog.Warn("write plot list cache", "key", key, "error", cacheErr)
		}
	}
	return result, err
}

func (s *cachedStore) FindByIDAndOwner(ctx context.Context, plotID, ownerID uint64) (*Plot, error) {
	version, err := s.cache.Version(ctx, plotVersionKey(ownerID))
	if err != nil {
		slog.Warn("read plot cache version", "ownerId", ownerID, "error", err)
		return s.next.FindByIDAndOwner(ctx, plotID, ownerID)
	}
	key := fmt.Sprintf("agri:v1:cache:plots:owner:%d:g:%d:id:%d", ownerID, version, plotID)
	var cached cachedPlotDetail
	if hit, cacheErr := s.cache.GetJSON(ctx, key, &cached); cacheErr == nil && hit && cached.SchemaVersion == cachex.SchemaVersion {
		return &cached.Item, nil
	} else if cacheErr != nil {
		slog.Warn("read plot detail cache", "key", key, "error", cacheErr)
	}
	value, err := s.next.FindByIDAndOwner(ctx, plotID, ownerID)
	if err == nil && value != nil {
		if cacheErr := s.cache.SetJSON(ctx, key, cachedPlotDetail{SchemaVersion: cachex.SchemaVersion, Item: *value}, s.ttl); cacheErr != nil {
			slog.Warn("write plot detail cache", "key", key, "error", cacheErr)
		}
	}
	return value, err
}

func (s *cachedStore) UpdateCrop(ctx context.Context, plotID, ownerID uint64, cropType string, plantingTime time.Time) error {
	if err := s.next.UpdateCrop(ctx, plotID, ownerID, cropType, plantingTime); err != nil {
		return err
	}
	s.bump(ctx, ownerID)
	return nil
}

func (s *cachedStore) bump(ctx context.Context, ownerID uint64) {
	if err := s.cache.BumpVersion(ctx, plotVersionKey(ownerID)); err != nil {
		slog.Warn("invalidate plot cache", "ownerId", ownerID, "error", err)
	}
}

func plotVersionKey(ownerID uint64) string {
	return fmt.Sprintf("agri:v1:cache:plots:owner:%d:version", ownerID)
}
