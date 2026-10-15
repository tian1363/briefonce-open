package assetextract

import (
	"context"
	"strings"
	"time"

	"github.com/tian1363/scriptagent/internal/model"
)

// Provider calls from one video batch share a small, context-aware lane.
// Spacing starts after each response, so the second analysis wave cannot
// immediately burst after the keyframe wave.
var visionLane = make(chan struct{}, 1)
var nextVisionCall time.Time

func callVisionWithRetry(ctx context.Context, call func() (model.Generation, error)) (model.Generation, error) {
	select {
	case visionLane <- struct{}{}:
		defer func() { <-visionLane }()
	case <-ctx.Done():
		return model.Generation{}, ctx.Err()
	}
	for attempt := 0; attempt < 4; attempt++ {
		if err := waitVision(ctx, time.Until(nextVisionCall)); err != nil {
			return model.Generation{}, err
		}
		result, err := call()
		nextVisionCall = time.Now().Add(time.Second)
		if err == nil || !isVisionThrottle(err) || attempt == 3 {
			return result, err
		}
		if err := waitVision(ctx, time.Duration(2<<attempt)*time.Second); err != nil {
			return model.Generation{}, err
		}
	}
	return model.Generation{}, ctx.Err()
}

func waitVision(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isVisionThrottle(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "模型请求过于频繁") || strings.Contains(message, "Throttling") || strings.Contains(message, "status 429")
}

func (s *Service) generateVision(ctx context.Context, callCtx model.CallContext, content []model.ContentItem) (model.Generation, error) {
	return callVisionWithRetry(ctx, func() (model.Generation, error) {
		return s.Vision.GenerateDetailed(ctx, callCtx, content)
	})
}
