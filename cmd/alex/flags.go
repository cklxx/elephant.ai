package main

import (
	"fmt"
	"strconv"
	"strings"

	"alex/internal/config"
)

type autoReviewCLIOverrides struct {
	enabled      *bool
	minScore     *float64
	enableRework *bool
	maxRework    *int
}

type globalCLIOptions struct {
	autoReview autoReviewCLIOverrides
}

func parseGlobalCLIOptions(args []string) (globalCLIOptions, []string, error) {
	opts := globalCLIOptions{}
	var filtered []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			filtered = append(filtered, arg)
			continue
		}

		consumed := false
		switch {
		case arg == "--auto-review":
			opts.autoReview.enabled = boolPtr(true)
			consumed = true
		case arg == "--no-auto-review":
			opts.autoReview.enabled = boolPtr(false)
			consumed = true
		case strings.HasPrefix(arg, "--auto-review="):
			value := strings.TrimPrefix(arg, "--auto-review=")
			b, err := parseBoolFlagValue(value)
			if err != nil {
				return opts, nil, err
			}
			opts.autoReview.enabled = boolPtr(b)
			consumed = true
		case strings.HasPrefix(arg, "--auto-review-min-score"):
			value, err := extractFlagValue(arg, args, &i)
			if err != nil {
				return opts, nil, err
			}
			score, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return opts, nil, fmt.Errorf("invalid value for --auto-review-min-score: %w", err)
			}
			if score <= 0 || score > 1 {
				return opts, nil, fmt.Errorf("--auto-review-min-score must be between 0 and 1")
			}
			opts.autoReview.minScore = &score
			consumed = true
		case strings.HasPrefix(arg, "--auto-review-rework"):
			value := ""
			if arg == "--auto-review-rework" {
				value = "true"
			}
			if value == "" {
				var err error
				value, err = extractFlagValue(arg, args, &i)
				if err != nil {
					return opts, nil, err
				}
			}
			b, err := parseBoolFlagValue(value)
			if err != nil {
				return opts, nil, err
			}
			opts.autoReview.enableRework = boolPtr(b)
			consumed = true
		case strings.HasPrefix(arg, "--auto-review-max-rework"):
			value, err := extractFlagValue(arg, args, &i)
			if err != nil {
				return opts, nil, err
			}
			attempts, err := strconv.Atoi(value)
			if err != nil {
				return opts, nil, fmt.Errorf("invalid value for --auto-review-max-rework: %w", err)
			}
			if attempts < 0 {
				return opts, nil, fmt.Errorf("--auto-review-max-rework must be >= 0")
			}
			opts.autoReview.maxRework = &attempts
			consumed = true
		}

		if !consumed {
			filtered = append(filtered, arg)
		}
	}

	return opts, filtered, nil
}

func (o globalCLIOptions) loaderOptions() []config.Option {
	overrides := config.Overrides{}
	has := false

	if o.autoReview.enabled != nil {
		overrides.AutoReviewEnabled = o.autoReview.enabled
		has = true
	}
	if o.autoReview.minScore != nil {
		overrides.AutoReviewMinPassingScore = o.autoReview.minScore
		has = true
	}
	if o.autoReview.enableRework != nil {
		overrides.AutoReviewEnableRework = o.autoReview.enableRework
		has = true
	}
	if o.autoReview.maxRework != nil {
		overrides.AutoReviewMaxReworkAttempts = o.autoReview.maxRework
		has = true
	}

	if !has {
		return nil
	}
	return []config.Option{config.WithOverrides(overrides)}
}

func extractFlagValue(current string, args []string, idx *int) (string, error) {
	if eq := strings.IndexByte(current, '='); eq != -1 {
		return current[eq+1:], nil
	}
	next := *idx + 1
	if next >= len(args) {
		return "", fmt.Errorf("flag %s requires a value", current)
	}
	*idx = next
	return args[next], nil
}

func parseBoolFlagValue(value string) (bool, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	switch lower {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
