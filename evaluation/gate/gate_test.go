package gate

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckThresholds_PassesWhenAboveMinimums(t *testing.T) {
	g := NewEvalGate()
	config := GateConfig{MinScore: 0.80, MinGrade: "B"}

	result := g.CheckThresholds(0.90, "A", config)

	assert.True(t, result.Passed)
	assert.Equal(t, 0.90, result.Score)
	assert.Equal(t, "A", result.Grade)
	assert.Empty(t, result.FailureReasons)
}

func TestCheckThresholds_PassesAtExactMinimum(t *testing.T) {
	g := NewEvalGate()
	config := GateConfig{MinScore: 0.80, MinGrade: "B"}

	result := g.CheckThresholds(0.80, "B", config)

	assert.True(t, result.Passed)
	assert.Empty(t, result.FailureReasons)
}

func TestCheckThresholds_FailsOnLowScore(t *testing.T) {
	g := NewEvalGate()
	config := GateConfig{MinScore: 0.80, MinGrade: "B"}

	result := g.CheckThresholds(0.50, "A", config)

	assert.False(t, result.Passed)
	require.Len(t, result.FailureReasons, 1)
	assert.Contains(t, result.FailureReasons[0], "score")
	assert.Contains(t, result.FailureReasons[0], "50.0%")
	assert.Contains(t, result.FailureReasons[0], "80.0%")
}

func TestCheckThresholds_FailsOnLowGrade(t *testing.T) {
	g := NewEvalGate()
	config := GateConfig{MinScore: 0.50, MinGrade: "B"}

	result := g.CheckThresholds(0.90, "C", config)

	assert.False(t, result.Passed)
	require.Len(t, result.FailureReasons, 1)
	assert.Contains(t, result.FailureReasons[0], "grade C")
	assert.Contains(t, result.FailureReasons[0], "minimum B")
}

func TestCheckThresholds_CollectsMultipleFailureReasons(t *testing.T) {
	g := NewEvalGate()
	config := GateConfig{MinScore: 0.80, MinGrade: "B"}

	result := g.CheckThresholds(0.50, "D", config)

	assert.False(t, result.Passed)
	require.Len(t, result.FailureReasons, 2)
	assert.Contains(t, result.FailureReasons[0], "score")
	assert.Contains(t, result.FailureReasons[1], "grade")
}

func TestGradeComparison(t *testing.T) {
	tests := []struct {
		grade    string
		min      string
		expected bool
	}{
		{"A", "A", true},
		{"A", "B", true},
		{"A", "C", true},
		{"A", "D", true},
		{"A", "F", true},
		{"B", "A", false},
		{"B", "B", true},
		{"B", "C", true},
		{"C", "B", false},
		{"C", "C", true},
		{"D", "C", false},
		{"D", "D", true},
		{"F", "D", false},
		{"F", "F", true},
	}

	for _, tc := range tests {
		t.Run(tc.grade+"_vs_"+tc.min, func(t *testing.T) {
			assert.Equal(t, tc.expected, gradeAtLeast(tc.grade, tc.min),
				"gradeAtLeast(%q, %q) should be %v", tc.grade, tc.min, tc.expected)
		})
	}
}

func TestGradeComparison_UnknownGrade(t *testing.T) {
	// Unknown grades have order 0, which is below any valid min grade.
	assert.False(t, gradeAtLeast("X", "F"))
	assert.False(t, gradeAtLeast("", "F"))
}

func TestFormatSummary_PassedContainsKeyFields(t *testing.T) {
	g := NewEvalGate()
	result := &GateResult{
		Passed:   true,
		Score:    0.92,
		Grade:    "A",
		Duration: 42 * time.Second,
	}

	summary := g.FormatSummary(result)

	assert.Contains(t, summary, "PASSED")
	assert.Contains(t, summary, "92.0%")
	assert.Contains(t, summary, "A")
	assert.Contains(t, summary, "42s")
	assert.NotContains(t, summary, "Failure Reasons")
}

func TestFormatSummary_FailedContainsReasons(t *testing.T) {
	g := NewEvalGate()
	result := &GateResult{
		Passed:         false,
		Score:          0.50,
		Grade:          "D",
		Duration:       3 * time.Minute,
		FailureReasons: []string{"score too low", "grade too low"},
	}

	summary := g.FormatSummary(result)

	assert.Contains(t, summary, "FAILED")
	assert.Contains(t, summary, "50.0%")
	assert.Contains(t, summary, "D")
	assert.Contains(t, summary, "Failure Reasons")
	assert.Contains(t, summary, "score too low")
	assert.Contains(t, summary, "grade too low")
}

func TestFormatSummary_NoDurationOmitsRow(t *testing.T) {
	g := NewEvalGate()
	result := &GateResult{
		Passed: true,
		Score:  0.85,
		Grade:  "B",
	}

	summary := g.FormatSummary(result)

	assert.NotContains(t, summary, "Duration")
}

func TestFormatSummary_IsValidMarkdown(t *testing.T) {
	g := NewEvalGate()
	result := &GateResult{
		Passed:         false,
		Score:          0.60,
		Grade:          "C",
		Duration:       2 * time.Minute,
		FailureReasons: []string{"below threshold"},
	}

	summary := g.FormatSummary(result)

	// Should contain markdown table header
	assert.Contains(t, summary, "| Metric | Value |")
	assert.Contains(t, summary, "|--------|-------|")
	// Should start with a markdown heading
	assert.True(t, strings.HasPrefix(summary, "## "))
}

func TestDefaultQuickGateConfig(t *testing.T) {
	config := DefaultQuickGateConfig()

	assert.Equal(t, 0.80, config.MinScore)
	assert.Equal(t, "B", config.MinGrade)
	assert.Equal(t, 5*time.Minute, config.MaxDuration)
	assert.Equal(t, 3, config.InstanceLimit)
	assert.Equal(t, 2, config.Workers)
	assert.NotEmpty(t, config.RequiredDataset)
}

func TestDefaultFullGateConfig(t *testing.T) {
	config := DefaultFullGateConfig()

	assert.Equal(t, 0.70, config.MinScore)
	assert.Equal(t, "C", config.MinGrade)
	assert.Equal(t, 30*time.Minute, config.MaxDuration)
	assert.Equal(t, 0, config.InstanceLimit) // 0 = all instances
	assert.Equal(t, 4, config.Workers)
	assert.NotEmpty(t, config.RequiredDataset)
}

func TestDefaultFullGateConfig_IsLessStrictThanQuick(t *testing.T) {
	quick := DefaultQuickGateConfig()
	full := DefaultFullGateConfig()

	assert.Greater(t, quick.MinScore, full.MinScore,
		"quick gate should require a higher minimum score")
	assert.True(t, gradeAtLeast(quick.MinGrade, full.MinGrade),
		"quick gate MinGrade should be at least as high as full gate MinGrade")
	assert.Greater(t, full.MaxDuration, quick.MaxDuration,
		"full gate should allow more time")
}

func TestGradeOrder_Transitivity(t *testing.T) {
	// Verify that A > B > C > D > F in gradeOrder.
	grades := []string{"A", "B", "C", "D", "F"}
	for i := 0; i < len(grades)-1; i++ {
		assert.Greater(t, gradeOrder[grades[i]], gradeOrder[grades[i+1]],
			"%s should rank higher than %s", grades[i], grades[i+1])
	}
}

func TestCheckThresholds_ZeroScoreAndGradeF(t *testing.T) {
	g := NewEvalGate()
	config := GateConfig{MinScore: 0.80, MinGrade: "B"}

	result := g.CheckThresholds(0.0, "F", config)

	assert.False(t, result.Passed)
	assert.Len(t, result.FailureReasons, 2)
}

func TestCheckThresholds_PerfectScoreAndGrade(t *testing.T) {
	g := NewEvalGate()
	config := GateConfig{MinScore: 0.80, MinGrade: "A"}

	result := g.CheckThresholds(1.0, "A", config)

	assert.True(t, result.Passed)
	assert.Empty(t, result.FailureReasons)
}
