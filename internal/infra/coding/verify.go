package coding

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	verifyKeyEnabled   = "verify"
	verifyKeyBuildCmd  = "verify_build_cmd"
	verifyKeyTestCmd   = "verify_test_cmd"
	verifyKeyLintCmd   = "verify_lint_cmd"
	defaultVerifyBuild = "go build ./..."
	defaultVerifyTest  = "go test ./..."
	defaultVerifyLint  = "./dev.sh lint"
)

// CommandRunner executes a command and returns combined output.
type CommandRunner interface {
	Run(ctx context.Context, workingDir string, command string) (string, error)
}

type shellCommandRunner struct{}

func (shellCommandRunner) Run(ctx context.Context, workingDir string, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = strings.TrimSpace(workingDir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ResolveVerificationPlan parses verification settings from task config.
// Verification is opt-in via verify=true/on/1/yes.
func ResolveVerificationPlan(config map[string]string) VerificationPlan {
	if len(config) == 0 {
		return VerificationPlan{}
	}

	if !isTruthy(config[verifyKeyEnabled]) {
		return VerificationPlan{}
	}

	plan := VerificationPlan{
		Enabled: true,
		Build:   strings.TrimSpace(config[verifyKeyBuildCmd]),
		Test:    strings.TrimSpace(config[verifyKeyTestCmd]),
		Lint:    strings.TrimSpace(config[verifyKeyLintCmd]),
	}

	if plan.Build == "" && plan.Test == "" && plan.Lint == "" {
		plan.Build = defaultVerifyBuild
		plan.Test = defaultVerifyTest
		plan.Lint = defaultVerifyLint
	}
	return plan
}

// VerifyBuild runs the build verification command.
func VerifyBuild(ctx context.Context, workingDir string, runner CommandRunner, command string) VerifyCheck {
	return runVerifyCheck(ctx, workingDir, runner, "build", command)
}

// VerifyTest runs the test verification command.
func VerifyTest(ctx context.Context, workingDir string, runner CommandRunner, command string) VerifyCheck {
	return runVerifyCheck(ctx, workingDir, runner, "test", command)
}

// VerifyLint runs the lint verification command.
func VerifyLint(ctx context.Context, workingDir string, runner CommandRunner, command string) VerifyCheck {
	return runVerifyCheck(ctx, workingDir, runner, "lint", command)
}

// VerifyAll runs build/test/lint verification checks in sequence.
func VerifyAll(ctx context.Context, workingDir string, runner CommandRunner, plan VerificationPlan) *VerifyResult {
	if !plan.Enabled {
		return nil
	}
	if runner == nil {
		runner = shellCommandRunner{}
	}

	result := &VerifyResult{
		Passed: true,
		Checks: []VerifyCheck{
			VerifyBuild(ctx, workingDir, runner, plan.Build),
			VerifyTest(ctx, workingDir, runner, plan.Test),
			VerifyLint(ctx, workingDir, runner, plan.Lint),
		},
	}

	for _, check := range result.Checks {
		if check.Skipped {
			continue
		}
		if !check.Passed {
			result.Passed = false
			break
		}
	}
	return result
}

func runVerifyCheck(ctx context.Context, workingDir string, runner CommandRunner, name string, command string) VerifyCheck {
	check := VerifyCheck{
		Name:    name,
		Command: strings.TrimSpace(command),
	}
	if check.Command == "" {
		check.Skipped = true
		check.Passed = true
		return check
	}
	if runner == nil {
		runner = shellCommandRunner{}
	}

	start := time.Now()
	out, err := runner.Run(ctx, workingDir, check.Command)
	check.Duration = time.Since(start)
	check.Output = strings.TrimSpace(out)
	if err != nil {
		check.Passed = false
		check.Error = err.Error()
		return check
	}
	check.Passed = true
	return check
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// VerifyError returns a compact error message for failed verification.
func VerifyError(result *VerifyResult) error {
	if result == nil || result.Passed {
		return nil
	}
	for _, check := range result.Checks {
		if check.Skipped || check.Passed {
			continue
		}
		if check.Error != "" {
			return fmt.Errorf("%s failed: %s", check.Name, check.Error)
		}
		return fmt.Errorf("%s failed", check.Name)
	}
	return fmt.Errorf("verification failed")
}
