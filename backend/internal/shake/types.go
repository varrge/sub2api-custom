// Package shake is the native lucky-box ("摇摇乐") system: campaigns, card
// issuance rules, cards, atomic draws and leaderboard reward jobs. It replaces
// the KaWang remote service; balances are credited in the draw transaction.
package shake

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound   = errors.New("shake: not found")
	ErrInvalid    = errors.New("shake: invalid input")
	ErrConflict   = errors.New("shake: state conflict")
	ErrClosed     = errors.New("shake: campaign not open")
	ErrNoCard     = errors.New("shake: no card available")
	ErrIneligible = errors.New("shake: personal accounts only")
)

var China = time.FixedZone("Asia/Shanghai", 8*60*60)

var Tiers = []string{"low", "medium", "high"}

const (
	SourceOrder        = "order"
	SourceUsage        = "usage"
	SourceManual       = "manual"
	SourceLeaderboard  = "leaderboard"
	SourceActivityTask = "activity_task"
	SourceExtraDraw    = "extra_draw"
)

// RedeemTypeShakeBalance marks balance-history rows written for balance prizes.
// It is deliberately not "balance"/"admin_balance" so it never counts as a recharge.
const RedeemTypeShakeBalance = "shake_balance"

func validTier(t string) bool { return t == "low" || t == "medium" || t == "high" }

// Rule is one operator-configured card issuance rule.
//
//   - source "order": completed payment orders of OrderType. Mode "per_order"
//     grants Quantity cards for every order; mode "threshold" accumulates the
//     order amount and grants Quantity cards per Threshold.
//   - source "usage": actual_cost of usage logs. Billing "balance" counts usage
//     not paid by a subscription; "subscription" counts subscription usage.
//     Always threshold mode.
//
// GroupID narrows a rule to one group. A rule without GroupID is the fallback
// and excludes groups that have their own rule of the same kind, so one
// order/usage row is counted by at most one rule.
type Rule struct {
	Source    string  `json:"source"`
	OrderType string  `json:"order_type,omitempty"`
	Billing   string  `json:"billing,omitempty"`
	Mode      string  `json:"mode"`
	GroupID   *int64  `json:"group_id,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Quantity  int     `json:"quantity"`
	CardTier  string  `json:"card_tier"`
}

// Kind identifies rules that compete for the same rows (used for fallbacks).
func (r Rule) Kind() string {
	if r.Source == SourceOrder {
		return "order:" + r.OrderType
	}
	return "usage:" + r.Billing
}

// Key is stable across config versions so progress survives rule edits.
func (r Rule) Key() string {
	scope := "all"
	if r.GroupID != nil {
		scope = fmt.Sprintf("g%d", *r.GroupID)
	}
	return r.Kind() + ":" + scope
}

type Weights struct {
	Low    float64 `json:"low"`
	Medium float64 `json:"medium"`
	High   float64 `json:"high"`
}

func (w Weights) For(tier string) float64 {
	switch tier {
	case "medium":
		return w.Medium
	case "high":
		return w.High
	}
	return w.Low
}

type Prize struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Amount      *float64 `json:"amount"`
	Weights     Weights  `json:"weights"`
	Rarity      string   `json:"rarity"`
	DisplayText string   `json:"display_text"`
	Icon        string   `json:"icon"`
	SortOrder   int      `json:"sort_order"`
}

// Config is the editable part of a campaign. Time windows are fixed once a
// campaign leaves draft.
type Config struct {
	Name     string    `json:"name"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Rules    []Rule    `json:"rules"`
	Prizes   []Prize   `json:"prizes"`
}

func finitePositive(v float64) bool { return v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v) }

// Normalize validates the config and fills prize IDs; it is idempotent.
func (c *Config) Normalize() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" || utf8.RuneCountInString(c.Name) > 100 {
		return fmt.Errorf("%w: 活动名称必填且不超过100字", ErrInvalid)
	}
	if c.StartsAt.IsZero() || !c.EndsAt.After(c.StartsAt) || c.EndsAt.Sub(c.StartsAt) > 366*24*time.Hour {
		return fmt.Errorf("%w: 结束时间必须晚于开始时间，且活动不超过366天", ErrInvalid)
	}
	if len(c.Rules) > 50 {
		return fmt.Errorf("%w: 最多50条发卡规则", ErrInvalid)
	}
	keys := map[string]bool{}
	for i := range c.Rules {
		r := &c.Rules[i]
		if r.Quantity == 0 {
			r.Quantity = 1
		}
		if !validTier(r.CardTier) || r.Quantity < 1 || r.Quantity > 100 {
			return fmt.Errorf("%w: 第%d条规则的卡种或张数无效", ErrInvalid, i+1)
		}
		if r.GroupID != nil && *r.GroupID <= 0 {
			return fmt.Errorf("%w: 第%d条规则的分组 ID 无效", ErrInvalid, i+1)
		}
		switch r.Source {
		case SourceOrder:
			if r.OrderType != "subscription" && r.OrderType != "month_card" && r.OrderType != "balance" {
				return fmt.Errorf("%w: 第%d条规则的订单类型无效", ErrInvalid, i+1)
			}
			r.Billing = ""
			if r.Mode != "per_order" && r.Mode != "threshold" {
				return fmt.Errorf("%w: 第%d条规则的发卡方式无效", ErrInvalid, i+1)
			}
		case SourceUsage:
			if r.Billing != "balance" && r.Billing != "subscription" {
				return fmt.Errorf("%w: 第%d条规则的计费类型无效", ErrInvalid, i+1)
			}
			r.OrderType = ""
			r.Mode = "threshold"
		default:
			return fmt.Errorf("%w: 第%d条规则的来源无效", ErrInvalid, i+1)
		}
		if r.Mode == "threshold" && !finitePositive(r.Threshold) {
			return fmt.Errorf("%w: 第%d条规则的阈值必须大于0", ErrInvalid, i+1)
		}
		if r.Mode == "per_order" {
			r.Threshold = 0
		}
		if keys[r.Key()] {
			return fmt.Errorf("%w: 同一来源和分组只能配置一条规则", ErrInvalid)
		}
		keys[r.Key()] = true
	}
	if len(c.Prizes) < 1 || len(c.Prizes) > 100 {
		return fmt.Errorf("%w: 奖池需要1-100个奖品", ErrInvalid)
	}
	ids := map[string]bool{}
	var totals Weights
	for i := range c.Prizes {
		p := &c.Prizes[i]
		p.Name = strings.TrimSpace(p.Name)
		p.DisplayText = strings.TrimSpace(p.DisplayText)
		p.Icon = strings.TrimSpace(p.Icon)
		if p.Name == "" || utf8.RuneCountInString(p.Name) > 100 || utf8.RuneCountInString(p.DisplayText) > 200 || len(p.Icon) > 500 {
			return fmt.Errorf("%w: 第%d个奖品的名称或展示文字无效", ErrInvalid, i+1)
		}
		switch p.Rarity {
		case "common", "rare", "epic", "legendary":
		default:
			return fmt.Errorf("%w: 第%d个奖品的稀有度无效", ErrInvalid, i+1)
		}
		switch p.Type {
		case "balance":
			if p.Amount == nil || !finitePositive(*p.Amount) || *p.Amount > 100000 {
				return fmt.Errorf("%w: 余额奖品金额必须大于0", ErrInvalid)
			}
		case "extra_draw", "empty":
			p.Amount = nil
		default:
			return fmt.Errorf("%w: 第%d个奖品的类型无效", ErrInvalid, i+1)
		}
		for _, w := range []float64{p.Weights.Low, p.Weights.Medium, p.Weights.High} {
			if w < 0 || math.IsInf(w, 0) || math.IsNaN(w) {
				return fmt.Errorf("%w: 奖品权重不能为负数", ErrInvalid)
			}
		}
		totals.Low += p.Weights.Low
		totals.Medium += p.Weights.Medium
		totals.High += p.Weights.High
		if p.ID == "" {
			p.ID = "p-" + randomHex(8)
		}
		if ids[p.ID] || len(p.ID) > 40 {
			return fmt.Errorf("%w: 奖品 ID 重复", ErrInvalid)
		}
		ids[p.ID] = true
	}
	for _, tier := range Tiers {
		if !(totals.For(tier) > 0) {
			return fmt.Errorf("%w: %s 抽奖卡至少需要一个权重大于0的奖品", ErrInvalid, tier)
		}
	}
	sort.SliceStable(c.Prizes, func(i, j int) bool { return c.Prizes[i].SortOrder < c.Prizes[j].SortOrder })
	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Campaign is the stored campaign with derived state.
type Campaign struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	StoredStatus  string     `json:"stored_status"`
	ConfigVersion int        `json:"config_version"`
	Config        Config     `json:"config"`
	CreatedAt     time.Time  `json:"created_at"`
	EndedAt       *time.Time `json:"ended_at"`
	EndReason     string     `json:"end_reason"`
}

// EffectiveStatus derives the user-visible phase from the stored status and clock.
func EffectiveStatus(stored string, start, end, now time.Time) string {
	switch stored {
	case "draft", "ended":
		return stored
	}
	if !now.Before(end) {
		return "ended"
	}
	if now.Before(start) {
		return "scheduled"
	}
	return "active"
}
