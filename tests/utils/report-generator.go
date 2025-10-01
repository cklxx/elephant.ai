package utils

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TestReport 测试报告结构
type TestReport struct {
	Metadata        ReportMetadata    `json:"metadata"`
	Summary         TestSummary       `json:"summary"`
	Suites          []TestSuite       `json:"suites"`
	Performance     PerformanceReport `json:"performance"`
	Coverage        CoverageReport    `json:"coverage"`
	Acceptance      AcceptanceReport  `json:"acceptance"`
	Recommendations []string          `json:"recommendations"`
}

// ReportMetadata 报告元数据
type ReportMetadata struct {
	GeneratedAt   time.Time `json:"generated_at"`
	Version       string    `json:"version"`
	Environment   string    `json:"environment"`
	GoVersion     string    `json:"go_version"`
	Platform      string    `json:"platform"`
	TestDuration  string    `json:"test_duration"`
	ReportVersion string    `json:"report_version"`
}

// TestSummary 测试摘要
type TestSummary struct {
	TotalTests    int     `json:"total_tests"`
	PassedTests   int     `json:"passed_tests"`
	FailedTests   int     `json:"failed_tests"`
	SkippedTests  int     `json:"skipped_tests"`
	PassRate      float64 `json:"pass_rate"`
	TotalDuration string  `json:"total_duration"`
	OverallStatus string  `json:"overall_status"`
}

// TestSuite 测试套件
type TestSuite struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Status      string       `json:"status"`
	Duration    string       `json:"duration"`
	Tests       []TestCase   `json:"tests"`
	Metrics     SuiteMetrics `json:"metrics"`
}

// TestCase 测试用例
type TestCase struct {
	Name     string            `json:"name"`
	Status   string            `json:"status"`
	Duration string            `json:"duration"`
	Output   string            `json:"output,omitempty"`
	Error    string            `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SuiteMetrics 套件指标
type SuiteMetrics struct {
	TestCount       int     `json:"test_count"`
	PassedCount     int     `json:"passed_count"`
	FailedCount     int     `json:"failed_count"`
	SkippedCount    int     `json:"skipped_count"`
	PassRate        float64 `json:"pass_rate"`
	AverageDuration string  `json:"average_duration"`
}

// PerformanceReport 性能报告
type PerformanceReport struct {
	Benchmarks    []BenchmarkResult  `json:"benchmarks"`
	LoadTests     []LoadTestResult   `json:"load_tests"`
	StressTests   []StressTestResult `json:"stress_tests"`
	MemoryProfile MemoryProfile      `json:"memory_profile"`
	CPUProfile    CPUProfile         `json:"cpu_profile"`
	Summary       PerformanceSummary `json:"summary"`
}

// BenchmarkResult 基准测试结果
type BenchmarkResult struct {
	Name           string  `json:"name"`
	Iterations     int64   `json:"iterations"`
	NsPerOp        int64   `json:"ns_per_op"`
	MBPerSec       float64 `json:"mb_per_sec"`
	AllocsPerOp    int64   `json:"allocs_per_op"`
	BytesPerOp     int64   `json:"bytes_per_op"`
	ComparedToBase float64 `json:"compared_to_base"`
}

// LoadTestResult 负载测试结果
type LoadTestResult struct {
	Name               string  `json:"name"`
	Concurrency        int     `json:"concurrency"`
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	RequestsPerSecond  float64 `json:"requests_per_second"`
	AverageLatency     string  `json:"average_latency"`
	P95Latency         string  `json:"p95_latency"`
	P99Latency         string  `json:"p99_latency"`
	SuccessRate        float64 `json:"success_rate"`
	Duration           string  `json:"duration"`
}

// StressTestResult 压力测试结果
type StressTestResult struct {
	Name            string              `json:"name"`
	MaxConcurrency  int                 `json:"max_concurrency"`
	Duration        string              `json:"duration"`
	TotalRequests   int64               `json:"total_requests"`
	SuccessRate     float64             `json:"success_rate"`
	SystemStability StabilityMetrics    `json:"system_stability"`
	ErrorBreakdown  map[string]int64    `json:"error_breakdown"`
	PhaseResults    []StressPhaseResult `json:"phase_results"`
}

// StabilityMetrics 稳定性指标
type StabilityMetrics struct {
	MemoryLeaks    bool   `json:"memory_leaks"`
	CrashOccurred  bool   `json:"crash_occurred"`
	RecoveryTime   string `json:"recovery_time"`
	MaxMemoryUsage string `json:"max_memory_usage"`
	MaxCPUUsage    string `json:"max_cpu_usage"`
}

// StressPhaseResult 压力测试阶段结果
type StressPhaseResult struct {
	Phase       string `json:"phase"`
	Duration    string `json:"duration"`
	Requests    int64  `json:"requests"`
	Errors      int64  `json:"errors"`
	MemoryUsage string `json:"memory_usage"`
}

// MemoryProfile 内存概况
type MemoryProfile struct {
	HeapAlloc    string `json:"heap_alloc"`
	HeapSys      string `json:"heap_sys"`
	HeapInuse    string `json:"heap_inuse"`
	HeapReleased string `json:"heap_released"`
	StackInuse   string `json:"stack_inuse"`
	GCRuns       uint32 `json:"gc_runs"`
	GCPauseTotal string `json:"gc_pause_total"`
}

// CPUProfile CPU概况
type CPUProfile struct {
	Samples      int           `json:"samples"`
	Duration     string        `json:"duration"`
	TopFunctions []CPUFunction `json:"top_functions"`
}

// CPUFunction CPU函数
type CPUFunction struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
	Samples int     `json:"samples"`
}

// PerformanceSummary 性能摘要
type PerformanceSummary struct {
	OverallRating          string             `json:"overall_rating"`
	PerformanceIssues      []PerformanceIssue `json:"performance_issues"`
	RecommendedActions     []string           `json:"recommended_actions"`
	ComparisonWithBaseline BaselineComparison `json:"comparison_with_baseline"`
}

// PerformanceIssue 性能问题
type PerformanceIssue struct {
	Severity    string `json:"severity"`
	Component   string `json:"component"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Suggestion  string `json:"suggestion"`
}

// BaselineComparison 基准对比
type BaselineComparison struct {
	BaselineVersion   string  `json:"baseline_version"`
	PerformanceChange float64 `json:"performance_change"`
	MemoryChange      float64 `json:"memory_change"`
	ThroughputChange  float64 `json:"throughput_change"`
	Summary           string  `json:"summary"`
}

// CoverageReport 覆盖率报告
type CoverageReport struct {
	OverallCoverage float64           `json:"overall_coverage"`
	PackageCoverage []PackageCoverage `json:"package_coverage"`
	FileCoverage    []FileCoverage    `json:"file_coverage"`
	UncoveredLines  []UncoveredLine   `json:"uncovered_lines"`
	CoverageGoals   CoverageGoals     `json:"coverage_goals"`
	Trend           CoverageTrend     `json:"trend"`
}

// PackageCoverage 包覆盖率
type PackageCoverage struct {
	Package    string  `json:"package"`
	Coverage   float64 `json:"coverage"`
	Statements int     `json:"statements"`
	Covered    int     `json:"covered"`
	Missing    int     `json:"missing"`
}

// FileCoverage 文件覆盖率
type FileCoverage struct {
	File       string  `json:"file"`
	Package    string  `json:"package"`
	Coverage   float64 `json:"coverage"`
	Statements int     `json:"statements"`
	Covered    int     `json:"covered"`
	Missing    int     `json:"missing"`
}

// UncoveredLine 未覆盖行
type UncoveredLine struct {
	File        string `json:"file"`
	LineNumber  int    `json:"line_number"`
	Function    string `json:"function"`
	Description string `json:"description"`
}

// CoverageGoals 覆盖率目标
type CoverageGoals struct {
	TargetCoverage   float64 `json:"target_coverage"`
	CurrentCoverage  float64 `json:"current_coverage"`
	GoalMet          bool    `json:"goal_met"`
	RequiredIncrease float64 `json:"required_increase"`
}

// CoverageTrend 覆盖率趋势
type CoverageTrend struct {
	PreviousCoverage float64 `json:"previous_coverage"`
	CurrentCoverage  float64 `json:"current_coverage"`
	Trend            string  `json:"trend"`
	Change           float64 `json:"change"`
}

// AcceptanceReport 验收报告
type AcceptanceReport struct {
	OverallAcceptance  AcceptanceStatus     `json:"overall_acceptance"`
	FunctionalTests    []AcceptanceCategory `json:"functional_tests"`
	PerformanceTests   []AcceptanceCategory `json:"performance_tests"`
	SecurityTests      []AcceptanceCategory `json:"security_tests"`
	UsabilityTests     []AcceptanceCategory `json:"usability_tests"`
	CompatibilityTests []AcceptanceCategory `json:"compatibility_tests"`
	Summary            AcceptanceSummary    `json:"summary"`
}

// AcceptanceStatus 验收状态
type AcceptanceStatus struct {
	Status      string    `json:"status"`
	Percentage  float64   `json:"percentage"`
	PassedTests int       `json:"passed_tests"`
	TotalTests  int       `json:"total_tests"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AcceptanceCategory 验收类别
type AcceptanceCategory struct {
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Tests       []AcceptanceTest  `json:"tests"`
	Metrics     AcceptanceMetrics `json:"metrics"`
}

// AcceptanceTest 验收测试
type AcceptanceTest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Notes       string `json:"notes,omitempty"`
}

// AcceptanceMetrics 验收指标
type AcceptanceMetrics struct {
	PassedCount int     `json:"passed_count"`
	TotalCount  int     `json:"total_count"`
	PassRate    float64 `json:"pass_rate"`
}

// AcceptanceSummary 验收摘要
type AcceptanceSummary struct {
	ReadyForProduction bool     `json:"ready_for_production"`
	CriticalIssues     []string `json:"critical_issues"`
	MinorIssues        []string `json:"minor_issues"`
	Recommendations    []string `json:"recommendations"`
	NextSteps          []string `json:"next_steps"`
}

// ReportGenerator 报告生成器
type ReportGenerator struct {
	outputDir string
	templates map[string]*template.Template
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(outputDir string) (*ReportGenerator, error) {
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	generator := &ReportGenerator{
		outputDir: outputDir,
		templates: make(map[string]*template.Template),
	}

	// 初始化模板
	err = generator.initTemplates()
	if err != nil {
		return nil, fmt.Errorf("初始化模板失败: %w", err)
	}

	return generator, nil
}

// GenerateReport 生成完整报告
func (rg *ReportGenerator) GenerateReport(report *TestReport) error {
	// 生成JSON报告
	err := rg.generateJSONReport(report)
	if err != nil {
		return fmt.Errorf("生成JSON报告失败: %w", err)
	}

	// 生成HTML报告
	err = rg.generateHTMLReport(report)
	if err != nil {
		return fmt.Errorf("生成HTML报告失败: %w", err)
	}

	// 生成Markdown报告
	err = rg.generateMarkdownReport(report)
	if err != nil {
		return fmt.Errorf("生成Markdown报告失败: %w", err)
	}

	// 生成CSV数据
	err = rg.generateCSVReport(report)
	if err != nil {
		return fmt.Errorf("生成CSV报告失败: %w", err)
	}

	return nil
}

// generateJSONReport 生成JSON报告
func (rg *ReportGenerator) generateJSONReport(report *TestReport) error {
	filename := filepath.Join(rg.outputDir, "test_report.json")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// generateHTMLReport 生成HTML报告
func (rg *ReportGenerator) generateHTMLReport(report *TestReport) error {
	filename := filepath.Join(rg.outputDir, "test_report.html")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	tmpl := rg.templates["html"]
	return tmpl.Execute(file, report)
}

// generateMarkdownReport 生成Markdown报告
func (rg *ReportGenerator) generateMarkdownReport(report *TestReport) error {
	filename := filepath.Join(rg.outputDir, "test_report.md")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	tmpl := rg.templates["markdown"]
	return tmpl.Execute(file, report)
}

// generateCSVReport 生成CSV数据
func (rg *ReportGenerator) generateCSVReport(report *TestReport) error {
	// 生成测试结果CSV
	err := rg.generateTestResultsCSV(report)
	if err != nil {
		return err
	}

	// 生成性能数据CSV
	err = rg.generatePerformanceCSV(report)
	if err != nil {
		return err
	}

	return nil
}

// generateTestResultsCSV 生成测试结果CSV
func (rg *ReportGenerator) generateTestResultsCSV(report *TestReport) error {
	filename := filepath.Join(rg.outputDir, "test_results.csv")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// 写入CSV标题
	_, err = file.WriteString("Suite,Test,Status,Duration,Error\n")
	if err != nil {
		return err
	}

	// 写入测试数据
	for _, suite := range report.Suites {
		for _, test := range suite.Tests {
			line := fmt.Sprintf("%s,%s,%s,%s,%s\n",
				suite.Name, test.Name, test.Status, test.Duration, strings.ReplaceAll(test.Error, ",", ";"))
			_, err = file.WriteString(line)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// generatePerformanceCSV 生成性能数据CSV
func (rg *ReportGenerator) generatePerformanceCSV(report *TestReport) error {
	filename := filepath.Join(rg.outputDir, "performance_data.csv")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// 写入CSV标题
	_, err = file.WriteString("Type,Name,Metric,Value,Unit\n")
	if err != nil {
		return err
	}

	// 写入基准测试数据
	for _, benchmark := range report.Performance.Benchmarks {
		lines := []string{
			fmt.Sprintf("Benchmark,%s,NsPerOp,%d,ns", benchmark.Name, benchmark.NsPerOp),
			fmt.Sprintf("Benchmark,%s,MBPerSec,%.2f,MB/s", benchmark.Name, benchmark.MBPerSec),
			fmt.Sprintf("Benchmark,%s,AllocsPerOp,%d,allocs", benchmark.Name, benchmark.AllocsPerOp),
		}
		for _, line := range lines {
			_, err = file.WriteString(line + "\n")
			if err != nil {
				return err
			}
		}
	}

	// 写入负载测试数据
	for _, loadTest := range report.Performance.LoadTests {
		lines := []string{
			fmt.Sprintf("LoadTest,%s,RequestsPerSecond,%.2f,RPS", loadTest.Name, loadTest.RequestsPerSecond),
			fmt.Sprintf("LoadTest,%s,SuccessRate,%.2f,%%", loadTest.Name, loadTest.SuccessRate),
			fmt.Sprintf("LoadTest,%s,AverageLatency,%s,duration", loadTest.Name, loadTest.AverageLatency),
		}
		for _, line := range lines {
			_, err = file.WriteString(line + "\n")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// initTemplates 初始化模板
func (rg *ReportGenerator) initTemplates() error {
	// HTML模板
	htmlTemplate := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ALEX 测试报告</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; margin: 0; padding: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { text-align: center; border-bottom: 3px solid #007acc; padding-bottom: 20px; margin-bottom: 30px; }
        .header h1 { color: #007acc; margin: 0; font-size: 2.5em; }
        .header .subtitle { color: #666; margin-top: 10px; font-size: 1.1em; }
        .metadata { background: #f8f9fa; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .summary-card { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 20px; border-radius: 10px; text-align: center; }
        .summary-card h3 { margin: 0 0 10px 0; font-size: 1.2em; }
        .summary-card .value { font-size: 2em; font-weight: bold; }
        .section { margin-bottom: 40px; }
        .section h2 { color: #333; border-bottom: 2px solid #007acc; padding-bottom: 10px; }
        .suite { background: #f8f9fa; border-left: 4px solid #007acc; padding: 15px; margin-bottom: 15px; border-radius: 5px; }
        .suite h3 { margin-top: 0; color: #007acc; }
        .test-case { background: white; margin: 10px 0; padding: 10px; border-radius: 5px; border-left: 3px solid #28a745; }
        .test-case.failed { border-left-color: #dc3545; }
        .test-case.skipped { border-left-color: #ffc107; }
        .status { padding: 3px 8px; border-radius: 12px; color: white; font-size: 0.9em; font-weight: bold; }
        .status.passed { background: #28a745; }
        .status.failed { background: #dc3545; }
        .status.skipped { background: #ffc107; color: #333; }
        .performance-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .performance-card { background: #f8f9fa; padding: 20px; border-radius: 10px; border: 1px solid #dee2e6; }
        .chart-placeholder { height: 200px; background: #e9ecef; border-radius: 5px; display: flex; align-items: center; justify-content: center; color: #6c757d; }
        .recommendations { background: #d1ecf1; border: 1px solid #bee5eb; padding: 15px; border-radius: 5px; }
        .recommendations ul { margin: 0; padding-left: 20px; }
        .footer { text-align: center; margin-top: 40px; padding-top: 20px; border-top: 1px solid #dee2e6; color: #6c757d; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>ALEX 测试报告</h1>
            <div class="subtitle">自动化测试和验收报告</div>
        </div>

        <div class="metadata">
            <p><strong>生成时间:</strong> {{.Metadata.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
            <p><strong>版本:</strong> {{.Metadata.Version}} | <strong>环境:</strong> {{.Metadata.Environment}} | <strong>Go版本:</strong> {{.Metadata.GoVersion}}</p>
            <p><strong>测试持续时间:</strong> {{.Metadata.TestDuration}}</p>
        </div>

        <div class="summary">
            <div class="summary-card">
                <h3>总测试数</h3>
                <div class="value">{{.Summary.TotalTests}}</div>
            </div>
            <div class="summary-card">
                <h3>通过率</h3>
                <div class="value">{{printf "%.1f%%" .Summary.PassRate}}</div>
            </div>
            <div class="summary-card">
                <h3>状态</h3>
                <div class="value">{{.Summary.OverallStatus}}</div>
            </div>
            <div class="summary-card">
                <h3>持续时间</h3>
                <div class="value">{{.Summary.TotalDuration}}</div>
            </div>
        </div>

        <div class="section">
            <h2>测试套件详情</h2>
            {{range .Suites}}
            <div class="suite">
                <h3>{{.Name}} - {{.Description}}</h3>
                <p><strong>状态:</strong> <span class="status {{.Status}}">{{.Status}}</span> |
                   <strong>持续时间:</strong> {{.Duration}} |
                   <strong>通过率:</strong> {{printf "%.1f%%" .Metrics.PassRate}}</p>
                {{range .Tests}}
                <div class="test-case {{.Status}}">
                    <strong>{{.Name}}</strong> <span class="status {{.Status}}">{{.Status}}</span>
                    <span style="float: right;">{{.Duration}}</span>
                    {{if .Error}}<br><small style="color: #dc3545;">{{.Error}}</small>{{end}}
                </div>
                {{end}}
            </div>
            {{end}}
        </div>

        <div class="section">
            <h2>性能测试结果</h2>
            <div class="performance-grid">
                <div class="performance-card">
                    <h4>负载测试</h4>
                    {{range .Performance.LoadTests}}
                    <p><strong>{{.Name}}</strong></p>
                    <p>并发数: {{.Concurrency}} | RPS: {{printf "%.2f" .RequestsPerSecond}} | 成功率: {{printf "%.2f%%" .SuccessRate}}</p>
                    {{end}}
                </div>
                <div class="performance-card">
                    <h4>内存使用</h4>
                    <p>堆分配: {{.Performance.MemoryProfile.HeapAlloc}}</p>
                    <p>系统内存: {{.Performance.MemoryProfile.HeapSys}}</p>
                    <p>GC次数: {{.Performance.MemoryProfile.GCRuns}}</p>
                </div>
            </div>
        </div>

        <div class="section">
            <h2>覆盖率报告</h2>
            <p><strong>总覆盖率:</strong> {{printf "%.2f%%" .Coverage.OverallCoverage}}</p>
            <p><strong>目标达成:</strong> {{if .Coverage.CoverageGoals.GoalMet}}✅ 已达到{{else}}❌ 未达到{{end}}</p>
        </div>

        <div class="section">
            <h2>验收状态</h2>
            <p><strong>总体状态:</strong> <span class="status {{.Acceptance.OverallAcceptance.Status}}">{{.Acceptance.OverallAcceptance.Status}}</span></p>
            <p><strong>完成度:</strong> {{printf "%.2f%%" .Acceptance.OverallAcceptance.Percentage}}</p>
            <p><strong>生产就绪:</strong> {{if .Acceptance.Summary.ReadyForProduction}}✅ 是{{else}}❌ 否{{end}}</p>
        </div>

        {{if .Recommendations}}
        <div class="section">
            <h2>建议和改进</h2>
            <div class="recommendations">
                <ul>
                    {{range .Recommendations}}
                    <li>{{.}}</li>
                    {{end}}
                </ul>
            </div>
        </div>
        {{end}}

        <div class="footer">
            <p>此报告由 ALEX 自动化测试系统生成</p>
        </div>
    </div>
</body>
</html>
`

	// Markdown模板
	markdownTemplate := `
# ALEX 测试报告

**生成时间:** {{.Metadata.GeneratedAt.Format "2006-01-02 15:04:05"}}
**版本:** {{.Metadata.Version}} | **环境:** {{.Metadata.Environment}}
**测试持续时间:** {{.Metadata.TestDuration}}

## 📊 测试摘要

| 指标 | 值 |
|------|-----|
| 总测试数 | {{.Summary.TotalTests}} |
| 通过测试 | {{.Summary.PassedTests}} |
| 失败测试 | {{.Summary.FailedTests}} |
| 跳过测试 | {{.Summary.SkippedTests}} |
| 通过率 | {{printf "%.2f%%" .Summary.PassRate}} |
| 总状态 | {{.Summary.OverallStatus}} |

## 🧪 测试套件详情

{{range .Suites}}
### {{.Name}} - {{.Description}}

**状态:** {{.Status}} | **持续时间:** {{.Duration}} | **通过率:** {{printf "%.1f%%" .Metrics.PassRate}}

{{range .Tests}}
- **{{.Name}}** - {{.Status}} ({{.Duration}}){{if .Error}}
  - 错误: {{.Error}}{{end}}
{{end}}

{{end}}

## 🚀 性能测试结果

### 负载测试
{{range .Performance.LoadTests}}
- **{{.Name}}**
  - 并发数: {{.Concurrency}}
  - 请求速率: {{printf "%.2f" .RequestsPerSecond}} RPS
  - 成功率: {{printf "%.2f%%" .SuccessRate}}
  - 平均延迟: {{.AverageLatency}}
{{end}}

### 内存使用
- 堆分配: {{.Performance.MemoryProfile.HeapAlloc}}
- 系统内存: {{.Performance.MemoryProfile.HeapSys}}
- GC次数: {{.Performance.MemoryProfile.GCRuns}}

## 📈 覆盖率报告

- **总覆盖率:** {{printf "%.2f%%" .Coverage.OverallCoverage}}
- **目标完成:** {{if .Coverage.CoverageGoals.GoalMet}}✅ 已达到{{else}}❌ 未达到{{end}}
- **趋势:** {{.Coverage.Trend.Trend}} ({{printf "%.2f%%" .Coverage.Trend.Change}})

## ✅ 验收状态

- **总体状态:** {{.Acceptance.OverallAcceptance.Status}}
- **完成度:** {{printf "%.2f%%" .Acceptance.OverallAcceptance.Percentage}}
- **生产就绪:** {{if .Acceptance.Summary.ReadyForProduction}}✅ 是{{else}}❌ 否{{end}}

{{if .Acceptance.Summary.CriticalIssues}}
### 关键问题
{{range .Acceptance.Summary.CriticalIssues}}
- {{.}}
{{end}}
{{end}}

{{if .Recommendations}}
## 💡 建议和改进

{{range .Recommendations}}
- {{.}}
{{end}}
{{end}}

---
*此报告由 ALEX 自动化测试系统生成*
`

	var err error
	rg.templates["html"], err = template.New("html").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	rg.templates["markdown"], err = template.New("markdown").Parse(markdownTemplate)
	if err != nil {
		return err
	}

	return nil
}

// CollectTestResults 收集测试结果
func CollectTestResults(logDir string) (*TestReport, error) {
	report := &TestReport{
		Metadata: ReportMetadata{
			GeneratedAt:   time.Now(),
			ReportVersion: "1.0.0",
		},
		Suites:          []TestSuite{},
		Recommendations: []string{},
	}

	// 扫描日志目录
	files, err := os.ReadDir(logDir)
	if err != nil {
		return nil, fmt.Errorf("读取日志目录失败: %w", err)
	}

	// 处理每个日志文件
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".log" {
			suiteName := strings.TrimSuffix(file.Name(), ".log")
			suite, err := parseTestLog(filepath.Join(logDir, file.Name()), suiteName)
			if err != nil {
				continue // 跳过解析失败的文件
			}
			report.Suites = append(report.Suites, *suite)
		}
	}

	// 计算摘要
	calculateSummary(report)

	// 生成建议
	generateRecommendations(report)

	return report, nil
}

// parseTestLog 解析测试日志
func parseTestLog(logFile, suiteName string) (*TestSuite, error) {
	file, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	suite := &TestSuite{
		Name:  suiteName,
		Tests: []TestCase{},
	}

	// 这里应该实现实际的日志解析逻辑
	// 简化实现，假设有结构化的日志格式

	return suite, nil
}

// calculateSummary 计算测试摘要
func calculateSummary(report *TestReport) {
	var totalTests, passedTests, failedTests, skippedTests int

	for _, suite := range report.Suites {
		for _, test := range suite.Tests {
			totalTests++
			switch test.Status {
			case "passed":
				passedTests++
			case "failed":
				failedTests++
			case "skipped":
				skippedTests++
			}
		}
	}

	report.Summary = TestSummary{
		TotalTests:   totalTests,
		PassedTests:  passedTests,
		FailedTests:  failedTests,
		SkippedTests: skippedTests,
	}

	if totalTests > 0 {
		report.Summary.PassRate = float64(passedTests) / float64(totalTests) * 100
	}

	if report.Summary.PassRate >= 95 {
		report.Summary.OverallStatus = "优秀"
	} else if report.Summary.PassRate >= 80 {
		report.Summary.OverallStatus = "良好"
	} else if report.Summary.PassRate >= 60 {
		report.Summary.OverallStatus = "及格"
	} else {
		report.Summary.OverallStatus = "需要改进"
	}
}

// generateRecommendations 生成建议
func generateRecommendations(report *TestReport) {
	recommendations := []string{}

	// 基于通过率的建议
	if report.Summary.PassRate < 80 {
		recommendations = append(recommendations, "测试通过率较低，建议优先修复失败的测试用例")
	}

	// 基于性能的建议
	if report.Performance.Summary.OverallRating == "Poor" {
		recommendations = append(recommendations, "性能测试结果不理想，建议进行性能优化")
	}

	// 基于覆盖率的建议
	if report.Coverage.OverallCoverage < 80 {
		recommendations = append(recommendations, "代码覆盖率偏低，建议增加更多测试用例")
	}

	report.Recommendations = recommendations
}

// GenerateComparisonReport 生成对比报告
func (rg *ReportGenerator) GenerateComparisonReport(current, baseline *TestReport) error {
	comparison := struct {
		Current  *TestReport `json:"current"`
		Baseline *TestReport `json:"baseline"`
		Changes  interface{} `json:"changes"`
	}{
		Current:  current,
		Baseline: baseline,
		Changes:  calculateChanges(current, baseline),
	}

	filename := filepath.Join(rg.outputDir, "comparison_report.json")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(comparison)
}

// calculateChanges 计算变化
func calculateChanges(current, baseline *TestReport) interface{} {
	return map[string]interface{}{
		"pass_rate_change": current.Summary.PassRate - baseline.Summary.PassRate,
		"coverage_change":  current.Coverage.OverallCoverage - baseline.Coverage.OverallCoverage,
	}
}
