package monthcard

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func coreProduct() Product {
	return Product{GroupID: 1, Name: "Independent month card", PriceCNY: 198, BaseQuotaUSD: 940, MaxMembers: 10, ForSale: true, Tiers: []Tier{{3, 960}, {6, 1000}, {10, 1100}}}
}

func TestCoreProductValidation(t *testing.T) {
	p := coreProduct()
	require.NoError(t, validateProduct(&p))
	require.Equal(t, 48, p.RecruitmentHours)
	for _, tc := range []struct {
		name   string
		mutate func(*Product)
	}{
		{"nan price", func(p *Product) { p.PriceCNY = math.NaN() }},
		{"infinite quota", func(p *Product) { p.BaseQuotaUSD = math.Inf(1) }},
		{"subcent price", func(p *Product) { p.PriceCNY = 198.001 }},
		{"precision loss", func(p *Product) { p.BaseQuotaUSD = 0.000000001 }},
		{"duplicate threshold", func(p *Product) { p.Tiers[1].Members = 3 }},
		{"decreasing quota", func(p *Product) { p.Tiers[1].QuotaUSD = 959 }},
		{"threshold above capacity", func(p *Product) { p.MaxMembers = 9 }},
		{"solo team", func(p *Product) { p.MaxMembers = 1 }},
		{"negative recruitment", func(p *Product) { p.RecruitmentHours = -1 }},
		{"missing group", func(p *Product) { p.GroupID = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := coreProduct()
			tc.mutate(&p)
			require.ErrorIs(t, validateProduct(&p), ErrInvalid)
		})
	}
}

func TestCoreQuotaThresholds(t *testing.T) {
	p := coreProduct()
	for members, want := range map[int]string{1: "940", 2: "940", 3: "960", 5: "960", 6: "1000", 9: "1000", 10: "1100"} {
		require.Equal(t, want, quotaForMembers(p, members).String())
	}
}
