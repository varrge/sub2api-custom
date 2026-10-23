package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type ActivityRankReward struct {
	Rank     int    `json:"rank"`
	CardTier string `json:"card_tier"`
	Quantity int    `json:"quantity"`
}

type ActivityParticipationReward struct {
	Enabled         bool   `json:"enabled"`
	CardTier        string `json:"card_tier"`
	Quantity        int    `json:"quantity"`
	IncludeZeroCost bool   `json:"include_zero_cost"`
}

type ActivityRewardRules struct {
	Ranked        []ActivityRankReward        `json:"ranked"`
	Participation ActivityParticipationReward `json:"participation"`
}

type ActivityRewardPolicy struct {
	Period       string              `json:"period"`
	DelayMinutes int                 `json:"delay_minutes"`
	Rewards      ActivityRewardRules `json:"rewards"`
}

func (c *ActivityLeaderboardConfig) RewardPolicy() ActivityRewardPolicy {
	p := ActivityRewardPolicy{Period: c.Period, DelayMinutes: 10, Rewards: ActivityRewardRules{
		Ranked:        []ActivityRankReward{{1, "high", 1}, {2, "medium", 2}, {3, "medium", 1}},
		Participation: ActivityParticipationReward{true, "low", 1, true},
	}}
	if p.Period == "" {
		p.Period = "daily"
	}
	if c.SettlementDelayMinutes != nil {
		p.DelayMinutes = *c.SettlementDelayMinutes
	}
	if c.RewardRules != nil {
		p.Rewards.Participation = c.RewardRules.Participation
		p.Rewards.Ranked = append([]ActivityRankReward{}, c.RewardRules.Ranked...)
	}
	sort.Slice(p.Rewards.Ranked, func(i, j int) bool { return p.Rewards.Ranked[i].Rank < p.Rewards.Ranked[j].Rank })
	return p
}

func (p ActivityRewardPolicy) Validate() error {
	if p.Period != "daily" && p.Period != "event" {
		return fmt.Errorf("invalid reward period")
	}
	if p.DelayMinutes < 10 || p.DelayMinutes > 1440 {
		return fmt.Errorf("settlement delay must be 10-1440 minutes")
	}
	if len(p.Rewards.Ranked) > 20 {
		return fmt.Errorf("at most 20 rank rewards are allowed")
	}
	validCard := func(tier string, quantity int) bool {
		return (tier == "low" || tier == "medium" || tier == "high") && quantity >= 1 && quantity <= 100
	}
	seen := map[int]bool{}
	for _, r := range p.Rewards.Ranked {
		if r.Rank < 1 || r.Rank > 1000 || seen[r.Rank] || !validCard(r.CardTier, r.Quantity) {
			return fmt.Errorf("rank rewards require unique ranks 1-1000, valid tiers and quantities 1-100")
		}
		seen[r.Rank] = true
	}
	r := p.Rewards.Participation
	if !validCard(r.CardTier, r.Quantity) {
		return fmt.Errorf("participation requires a valid tier and quantity 1-100")
	}
	return nil
}

func (p ActivityRewardPolicy) Fingerprint() string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (p ActivityRewardPolicy) Award(rank int) (string, int) {
	if rank > 0 {
		for _, r := range p.Rewards.Ranked {
			if r.Rank == rank {
				return r.CardTier, r.Quantity
			}
		}
	}
	r := p.Rewards.Participation
	if r.Enabled && (rank > 0 || r.IncludeZeroCost) {
		return r.CardTier, r.Quantity
	}
	return "", 0
}
