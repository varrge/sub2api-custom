package groupeval

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluationValidation(t *testing.T) {
	base := DefaultConfig(1, "Test")
	base.Model, base.Enabled = "gpt-test", true
	require.NoError(t, base.Validate())
	for _, change := range []func(*Config){
		func(c *Config) { c.IntervalSeconds = 9 }, func(c *Config) { c.IntervalSeconds = 86401 },
		func(c *Config) { c.Model = "" }, func(c *Config) { c.Effort = "invalid" },
		func(c *Config) { c.Runs = 11 }, func(c *Config) { c.DailyLimit = 2 },
		func(c *Config) { c.CandyEnabled = false },
	} {
		c := base
		change(&c)
		require.ErrorIs(t, c.Validate(), ErrInvalid)
	}
}

func TestCandyUsesFinalAnswerNotIncidentalNumbers(t *testing.T) {
	for _, tc := range []struct{ output, status, answer string }{
		{"Blind selection would require 29.\nFINAL_ANSWER=21", "passed", "21"},
		{"21 appears in the derivation.\nFINAL_ANSWER=29", "mismatch", "29"},
		{"21", "error", ""},
		{"FINAL_ANSWER=29\nNo, maybe not.", "error", ""},
		{"FINAL_ANSWER=29\nFINAL_ANSWER=21", "error", ""},
		{"FINAL_ANSWER=" + strings.Repeat("1", 8193), "error", ""},
		{strings.Repeat("x", MaxOutputBytes+1), "error", ""},
	} {
		status, answer, html, _ := Evaluate("candy", tc.output)
		require.Equal(t, tc.status, status)
		require.Equal(t, tc.answer, answer)
		require.Empty(t, html)
	}
}

func TestCandyReferenceAnswerExhaustive(t *testing.T) {
	// Mark every achievable (round, star) draw that can still avoid a pair.
	unsafe := map[[2]int]bool{}
	for appleRound := 0; appleRound <= 7; appleRound++ {
		for peachRound := 0; peachRound <= 9; peachRound++ {
			for appleStar := 0; appleStar <= 7; appleStar++ {
				for peachStar := 0; peachStar <= 6; peachStar++ {
					if appleRound > 0 && peachStar > 0 || peachRound > 0 && appleStar > 0 {
						continue
					}
					for watermelonRound := 0; watermelonRound <= 8; watermelonRound++ {
						for watermelonStar := 0; watermelonStar <= 4; watermelonStar++ {
							unsafe[[2]int{appleRound + peachRound + watermelonRound, appleStar + peachStar + watermelonStar}] = true
						}
					}
				}
			}
		}
	}
	minimum := 100
	for round := 0; round <= 24; round++ {
		for star := 0; star <= 17; star++ {
			if !unsafe[[2]int{round, star}] {
				minimum = min(minimum, round+star)
			}
		}
	}
	require.Equal(t, 21, minimum)
	require.False(t, unsafe[[2]int{9, 12}])
}

func TestPelicanRequiresCompleteBoundedMarkup(t *testing.T) {
	status, _, html, _ := Evaluate("pelican", "```html\n<!doctype html><html><body>sample</body></html>\n```")
	require.Equal(t, "generated", status)
	require.True(t, strings.HasPrefix(html, "<!doctype"))
	for _, input := range []string{"not html", "<html>truncated", "<html>\x00</html>", "```html\n<html></html>", strings.Repeat("x", MaxOutputBytes+1)} {
		status, _, html, _ = Evaluate("pelican", input)
		require.Equal(t, "error", status)
		require.Empty(t, html)
	}
}
