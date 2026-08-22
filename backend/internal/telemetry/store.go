package telemetry

import (
	"context"
	"errors"
)

// ErrNotFound 表示地块暂无遥测数据。
var ErrNotFound = errors.New("telemetry not found")

// Store 抽象遥测最新值的读取，未来由 Redis 实现。
type Store interface {
	LatestByPlot(ctx context.Context, plotID uint64) (*Latest, error)
	// LatestByPlots 批量返回多个地块的最新遥测；未接入遥测的地块不出现在结果中。
	LatestByPlots(ctx context.Context, plotIDs []uint64) ([]Latest, error)
}

// NullStore 在遥测尚未接入（Redis/TDengine 未实现）时使用，恒返回 ErrNotFound。
type NullStore struct{}

func (NullStore) LatestByPlot(_ context.Context, _ uint64) (*Latest, error) {
	return nil, ErrNotFound
}

func (NullStore) LatestByPlots(_ context.Context, _ []uint64) ([]Latest, error) {
	return []Latest{}, nil
}
