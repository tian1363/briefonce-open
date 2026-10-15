package model

import (
	"context"
	"errors"
)

// UsageCharger is implemented by the store. Provider requests never receive a
// platform credential until a managed account has reserved its points.
type UsageCharger interface {
	ReserveUsage(ctx context.Context, capability string, units int) (string, error)
	FinishUsage(chargeID string, success bool) error
}

func (c *DashScopeClient) beginUsage(ctx context.Context, runtime RuntimeConfig, capability string, units int, callCtx CallContext) (func(bool), error) {
	if runtime.Source != "managed" {
		return func(bool) {}, nil
	}
	charger, ok := c.provider.(UsageCharger)
	if !ok {
		return nil, errors.New("平台积分服务不可用")
	}
	id, err := charger.ReserveUsage(ctx, capability, units)
	if err != nil {
		return nil, err
	}
	if describer, ok := c.provider.(interface {
		DescribeUsage(context.Context, string, string, string, string) error
	}); ok {
		label := callCtx.TraceName
		if label == "" {
			label = callCtx.Step
		}
		if err := describer.DescribeUsage(ctx, id, runtime.Model, callCtx.RefID, label); err != nil {
			_ = charger.FinishUsage(id, false)
			return nil, err
		}
	}
	return func(success bool) { _ = charger.FinishUsage(id, success) }, nil
}
