package assetextract

import (
	"context"
	"errors"
	"testing"

	"github.com/tian1363/scriptagent/internal/model"
)

func TestVisionBurstRateRetries(t *testing.T) {
	attempts := 0
	result, err := callVisionWithRetry(context.Background(), func() (model.Generation, error) {
		attempts++
		if attempts == 1 {
			return model.Generation{}, errors.New("模型请求过于频繁，请稍后重试")
		}
		return model.Generation{Text: "ok"}, nil
	})
	if err != nil || result.Text != "ok" || attempts != 2 {
		t.Fatalf("retry result = %+v, %v, attempts=%d", result, err, attempts)
	}
}

func TestVisionGateHonorsCancellation(t *testing.T) {
	visionLane <- struct{}{}
	defer func() { <-visionLane }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := callVisionWithRetry(ctx, func() (model.Generation, error) {
		t.Fatal("called provider after cancellation")
		return model.Generation{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
