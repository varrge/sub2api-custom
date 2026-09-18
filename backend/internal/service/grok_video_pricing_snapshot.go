package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// GrokVideoPricingSnapshot freezes the effective pricing rule at submission.
// Per-request channel prices and per-second video prices retain their different
// unit semantics when status later supplies the actual output duration.
type GrokVideoPricingSnapshot struct {
	UnitPrice                 float64              `json:"unit_price"`
	PerSecond                 bool                 `json:"per_second"`
	RateMultiplier            float64              `json:"rate_multiplier"`
	Source                    string               `json:"source"`
	PricingAt                 time.Time            `json:"pricing_at"`
	TokenPricing              *ResolvedPricing     `json:"token_pricing,omitempty"`
	TokenChannelPricing       *ChannelModelPricing `json:"token_channel_pricing,omitempty"`
	LongContextPricingEnabled bool                 `json:"long_context_pricing_enabled,omitempty"`
}

func (s *OpenAIGatewayService) SnapshotGrokVideoPricing(ctx context.Context, key *APIKey, model, resolution string, at time.Time) (*GrokVideoPricingSnapshot, error) {
	if s == nil || s.billingService == nil || key == nil {
		return nil, fmt.Errorf("video pricing unavailable")
	}
	resolution = NormalizeVideoBillingResolutionOrDefault(resolution)
	snapshot := &GrokVideoPricingSnapshot{PerSecond: true, RateMultiplier: s.ResolveVideoRateMultiplierAt(ctx, key, key.UserID, at), Source: PricingSourceFallback, PricingAt: at}
	resolved := s.resolveOpenAIChannelPricing(ctx, model, key)
	if resolved != nil && resolved.Mode == BillingModeToken {
		// JSON cloning freezes nested intervals, price pointers and channel time
		// rules; retaining cached resolver pointers would let later edits leak in.
		snapshot.TokenPricing = resolved
		snapshot.TokenChannelPricing = resolved.channelPricing
		snapshot.LongContextPricingEnabled = resolved.longContextPricingEnabled
		snapshot.RateMultiplier, _ = computePeakAwareMultipliers(key, s.resolveRateMultiplierAt(ctx, key, key.UserID, at), at)
		payload, err := json.Marshal(snapshot)
		if err != nil {
			return nil, err
		}
		var frozen GrokVideoPricingSnapshot
		if err := json.Unmarshal(payload, &frozen); err != nil {
			return nil, err
		}
		return &frozen, nil
	}
	fromResolved := func(pricing *ResolvedPricing) (*GrokVideoPricingSnapshot, error) {
		cost, err := s.billingService.CalculateCostUnified(CostInput{Ctx: ctx, Model: model, GroupID: key.GroupID, Group: key.Group, UsageUnits: 1, SizeTier: resolution, RateMultiplier: 1, Resolver: s.resolver, Resolved: pricing, PricingAt: at})
		if err != nil {
			return nil, err
		}
		snapshot.UnitPrice, snapshot.PerSecond, snapshot.Source = cost.TotalCost, pricing.Mode == BillingModeVideo, pricing.Source
		return snapshot, nil
	}
	if resolved != nil && resolved.Source == PricingSourceGroup && resolved.Mode == BillingModeVideo {
		return fromResolved(resolved)
	}
	if apiKeyHasConfiguredVideoPrice(key, model, resolution) {
		snapshot.Source = PricingSourceGroup
		snapshot.UnitPrice = s.billingService.getVideoUnitPrice(model, resolution, videoPriceConfigFromAPIKey(key))
		return snapshot, nil
	}
	if refreshed := s.apiKeyWithFreshGroupMediaPricing(ctx, key); refreshed != key {
		key = refreshed
		if apiKeyHasConfiguredVideoPrice(key, model, resolution) {
			snapshot.Source = PricingSourceGroup
			snapshot.UnitPrice = s.billingService.getVideoUnitPrice(model, resolution, videoPriceConfigFromAPIKey(key))
			return snapshot, nil
		}
	}
	if resolved != nil && resolved.Source == PricingSourceChannel && (resolved.Mode == BillingModePerRequest || resolved.Mode == BillingModeImage || resolved.Mode == BillingModeVideo) {
		return fromResolved(resolved)
	}
	snapshot.UnitPrice = s.billingService.getVideoUnitPrice(model, resolution, videoPriceConfigFromAPIKey(key))
	return snapshot, nil
}

func (s *GrokVideoPricingSnapshot) calculate(ctx context.Context, billing *BillingService, model string, result *OpenAIForwardResult, tokens UsageTokens, serviceTier string) (*CostBreakdown, error) {
	if s.TokenPricing != nil {
		resolved := *s.TokenPricing
		resolved.channelPricing = s.TokenChannelPricing
		resolved.longContextPricingEnabled = s.LongContextPricingEnabled
		return billing.CalculateCostUnified(CostInput{Ctx: ctx, Model: model, Tokens: tokens, ServiceTier: serviceTier, RateMultiplier: s.RateMultiplier, PricingAt: s.PricingAt, Resolved: &resolved, Resolver: NewModelPricingResolver(nil, billing)})
	}
	units := result.VideoCount
	if units <= 0 {
		units = 1
	}
	if s.PerSecond {
		units *= NormalizeVideoBillingDurationSecondsOrDefault(result.VideoDurationSeconds)
	}
	total := s.UnitPrice * float64(units)
	return &CostBreakdown{TotalCost: total, ActualCost: total * s.RateMultiplier, BillingMode: string(BillingModeVideo)}, nil
}
