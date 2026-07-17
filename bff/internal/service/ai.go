package service

import (
	"context"
	"sort"
	"strings"
)

// Analyzer abstracts log analysis (rule-based now, AI later).
type Analyzer interface {
	Analyze(ctx context.Context, log *LogResult) (*AnalyzeResult, error)
}

// AnalyzeResult is the AI / rule analysis response.
type AnalyzeResult struct {
	Mode         string           `json:"mode"`
	Summary      string           `json:"summary"`
	ModuleErrors []ModuleError    `json:"moduleErrors"`
	Suggestions  []Suggestion     `json:"suggestions"`
}

type ModuleError struct {
	Module string `json:"module"`
	Count  int    `json:"count"`
}

type Suggestion struct {
	Code     string `json:"code"`
	Desc     string `json:"desc"`
	Severity string `json:"severity"`
}

// RuleAnalyzer is the placeholder analyzer using keyword/error-code heuristics.
type RuleAnalyzer struct{}

func NewRuleAnalyzer() *RuleAnalyzer { return &RuleAnalyzer{} }

func (a *RuleAnalyzer) Analyze(_ context.Context, log *LogResult) (*AnalyzeResult, error) {
	info, warn, err := 0, 0, 0
	modules := make(map[string]int)
	for _, l := range log.Lines {
		switch l.Level {
		case LevelInfo:
			info++
		case LevelWarn:
			warn++
		case LevelError:
			err++
			modules[detectModule(l.Msg)]++
		}
	}
	res := &AnalyzeResult{
		Mode:    "rule",
		Summary: summaryLine(len(log.Lines), info, warn, err),
	}
	for m, c := range modules {
		res.ModuleErrors = append(res.ModuleErrors, ModuleError{Module: m, Count: c})
	}
	sort.Slice(res.ModuleErrors, func(i, j int) bool { return res.ModuleErrors[i].Count > res.ModuleErrors[j].Count })
	res.Suggestions = ruleSuggestions(modules)
	return res, nil
}

func summaryLine(total, info, warn, errc int) string {
	return strings.Join([]string{
		"共 " + itoa(total) + " 行",
		"Info " + itoa(info) + " 条",
		"Warn " + itoa(warn) + " 条",
		"Error " + itoa(errc) + " 条",
	}, "，")
}

// detectModule guesses the failing module from an error line by keyword.
func detectModule(msg string) string {
	low := strings.ToLower(msg)
	rules := []struct{ keyword, module string }{
		{"io", "IO"},
		{"disk", "IO"},
		{"permission", "IO"},
		{"parse", "Parse"},
		{"syntax", "Parse"},
		{"network", "Network"},
		{"timeout", "Network"},
		{"connection", "Network"},
		{"memory", "Resource"},
		{"oom", "Resource"},
		{"config", "Config"},
		{"license", "License"},
	}
	for _, r := range rules {
		if strings.Contains(low, r.keyword) {
			return r.module
		}
	}
	return "Other"
}

func ruleSuggestions(modules map[string]int) []Suggestion {
	var out []Suggestion
	if modules["IO"] > 0 {
		out = append(out, Suggestion{Code: "E_IO_001", Desc: "IO 模块出现读取/写入失败，建议检查磁盘空间、文件权限与挂载状态", Severity: "high"})
	}
	if modules["Network"] > 0 {
		out = append(out, Suggestion{Code: "E_NET_001", Desc: "网络相关错误，建议检查节点间连通性、端口与防火墙", Severity: "high"})
	}
	if modules["Parse"] > 0 {
		out = append(out, Suggestion{Code: "E_PARSE_001", Desc: "解析类错误，建议核对输入数据格式与参数配置", Severity: "medium"})
	}
	if modules["Resource"] > 0 {
		out = append(out, Suggestion{Code: "E_RES_001", Desc: "资源类错误，建议检查内存配额与节点负载", Severity: "high"})
	}
	if modules["Config"] > 0 {
		out = append(out, Suggestion{Code: "E_CFG_001", Desc: "配置类错误，建议核对作业参数与配置文件", Severity: "medium"})
	}
	if len(out) == 0 && modules["Other"] > 0 {
		out = append(out, Suggestion{Code: "E_GEN_001", Desc: "存在未归类错误，建议人工查看日志详情", Severity: "low"})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
