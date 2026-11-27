// Package groupeval implements administrator-funded group diagnostics.
package groupeval

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const HistoryLimit = 10
const MaxOutputBytes = 64 << 10

var (
	ErrInvalid  = errors.New("invalid evaluation configuration")
	ErrNotFound = errors.New("group evaluation not found")
	ErrConflict = errors.New("evaluation configuration changed")
	ErrBusy     = errors.New("evaluation is already running or capacity is busy")
	ErrDisabled = errors.New("group evaluation is disabled")
	ErrBudget   = errors.New("daily evaluation request limit reached")
)

type Config struct {
	GroupID         int64      `json:"group_id"`
	GroupName       string     `json:"group_name"`
	GroupPlatform   string     `json:"group_platform"`
	Revision        int64      `json:"revision"`
	Enabled         bool       `json:"enabled"`
	CandyEnabled    bool       `json:"candy_enabled"`
	PelicanEnabled  bool       `json:"pelican_enabled"`
	Automatic       bool       `json:"automatic"`
	IntervalSeconds int        `json:"interval_seconds"`
	Model           string     `json:"model"`
	Effort          string     `json:"effort"`
	Runs            int        `json:"runs"`
	DailyLimit      int        `json:"daily_limit"`
	LastRunAt       *time.Time `json:"last_run_at"`
	NextRunAt       *time.Time `json:"next_run_at"`
	Running         bool       `json:"running"`
}

func DefaultConfig(id int64, name string) Config {
	return Config{GroupID: id, GroupName: name, CandyEnabled: true, IntervalSeconds: 600, Effort: "low", Runs: 3, DailyLimit: 100}
}

func (c *Config) Validate() error {
	c.Model = strings.TrimSpace(c.Model)
	if c.GroupID <= 0 || c.Revision < 0 || c.IntervalSeconds < 10 || c.IntervalSeconds > 86400 ||
		c.Runs < 1 || c.Runs > 10 || c.DailyLimit < 1 || c.DailyLimit > 1000 ||
		(c.Effort != "low" && c.Effort != "medium" && c.Effort != "high") ||
		!utf8.ValidString(c.Model) || len(c.Model) > 200 || strings.ContainsAny(c.Model, "\r\n\x00") ||
		(c.Enabled && (c.Model == "" || (!c.CandyEnabled && !c.PelicanEnabled))) {
		return ErrInvalid
	}
	if c.Enabled && c.RequestsPerRun() > c.DailyLimit {
		return fmt.Errorf("%w: daily limit must cover at least one run", ErrInvalid)
	}
	return nil
}

func (c Config) RequestsPerRun() int {
	count := 0
	if c.CandyEnabled {
		count += c.Runs
	}
	if c.PelicanEnabled {
		count++
	}
	return count
}

// Public omits internal revision and budget fields from user-facing responses.
func (c Config) Public() map[string]any {
	return map[string]any{
		"group_id": c.GroupID, "group_name": c.GroupName, "group_platform": c.GroupPlatform, "enabled": c.Enabled,
		"candy_enabled": c.CandyEnabled, "pelican_enabled": c.PelicanEnabled,
		"automatic": c.Automatic, "interval_seconds": c.IntervalSeconds,
		"model": c.Model, "effort": c.Effort, "last_run_at": c.LastRunAt,
		"next_run_at": c.NextRunAt, "running": c.Running,
	}
}

type Record struct {
	ID             int64     `json:"id"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	Model          string    `json:"model"`
	Effort         string    `json:"effort"`
	CreatedAt      time.Time `json:"created_at"`
	Answer         string    `json:"answer"`
	HTML           string    `json:"html"`
	Error          string    `json:"error"`
	DurationMS     int64     `json:"duration_ms"`
	ExpectedAnswer string    `json:"expected_answer"`
	PromptVersion  string    `json:"prompt_version"`
}

// The original candy problem comes from CPA Codex Candy Eval; see NOTICE.md.
const CandyPrompt = `不使用任何外部工具回答以下问题：
在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。现已知不同口味的糖和不同形状的数量统计如下表。参赛者需要在活动前决定摸出的糖果数目，那么，最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（同时手中有圆形苹果味匹配五角星桃子味糖果，或者有圆形桃子味匹配五角星苹果味糖果都满足要求）
        苹果味  桃子味  西瓜味
圆形       7      9      8
五角星形   7      6      4
可以利用手感选择每种形状的取出数量，但不能辨别或选择口味；不是不分形状的随机盲取。请给出保证成功的最少总数。
可以先解释推理，最后一行必须仅写 FINAL_ANSWER=整数（不要代码块或 Markdown），例如 FINAL_ANSWER=0。`

const PelicanPrompt = `请直接输出一个完整的 HTML 文档，使用内联 SVG 和 CSS 绘制“鹈鹕骑自行车”的二维画面，可使用 CSS 动画。不使用任何工具，不创建文件，不使用 JavaScript、外部图片、字体、链接或网络资源。将完整文档放在一个 html 代码块中。`

var finalAnswer = regexp.MustCompile(`(?m)^FINAL_ANSWER=([0-9]+)\s*$`)

func Evaluate(kind, output string) (status, answer, html, publicError string) {
	if len(output) == 0 || len(output) > MaxOutputBytes || !utf8.ValidString(output) || strings.ContainsRune(output, '\x00') {
		return "error", "", "", "响应为空或超过检测输出上限"
	}
	if kind == "candy" {
		matches := finalAnswer.FindAllStringSubmatch(strings.TrimSpace(output), -1)
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(matches) != 1 || !finalAnswer.MatchString(lines[len(lines)-1]) {
			return "error", "", "", "未返回可判定的最终答案"
		}
		answer = matches[0][1]
		if len(answer) > 20 {
			return "error", "", "", "最终答案格式无效"
		}
		if answer == "21" {
			return "passed", answer, "", ""
		}
		return "mismatch", answer, "", ""
	}
	if kind != "pelican" {
		return "error", "", "", "不支持的检测项目"
	}
	code := strings.TrimSpace(output)
	if strings.HasPrefix(code, "```") {
		_, code, _ = strings.Cut(code, "\n")
		end := strings.LastIndex(code, "```")
		if end < 0 {
			return "error", "", "", "生成页面不完整"
		}
		code = strings.TrimSpace(code[:end])
	}
	lower := strings.ToLower(code)
	isHTML := (strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html")) && strings.HasSuffix(lower, "</html>")
	isSVG := strings.HasPrefix(lower, "<svg") && strings.HasSuffix(lower, "</svg>")
	if !isHTML && !isSVG {
		return "error", "", "", "未返回完整的 HTML 或 SVG"
	}
	// Raw markup is never served as an HTML document. The UI sanitizes and
	// isolates it in a sandbox with no script execution or network access.
	return "generated", "", code, ""
}
