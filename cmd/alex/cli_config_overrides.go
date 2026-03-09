package main

import (
	"fmt"
	"strconv"
	"strings"

	runtimeconfig "alex/internal/shared/config"
)

type overrideFieldHandler struct {
	set   func(*runtimeconfig.Overrides, string) error
	clear func(*runtimeconfig.Overrides)
}

var overrideFieldHandlers = map[string]overrideFieldHandler{
	"llm_provider":              stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.LLMProvider = v }),
	"llm_model":                 stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.LLMModel = v }),
	"llm_vision_model":          stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.LLMVisionModel = v }),
	"api_key":                   stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.APIKey = v }),
	"ark_api_key":               stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.ArkAPIKey = v }),
	"base_url":                  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.BaseURL = v }),
	"tavily_api_key":            stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.TavilyAPIKey = v }),
	"seedream_text_endpoint_id": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamTextEndpointID = v }),
	"seedream_image_endpoint_id": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) {
		o.SeedreamImageEndpointID = v
	}),
	"seedream_text_model":   stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamTextModel = v }),
	"seedream_image_model":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamImageModel = v }),
	"seedream_vision_model": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamVisionModel = v }),
	"seedream_video_model":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamVideoModel = v }),
	"profile": normalizedStringOverrideField(
		runtimeconfig.NormalizeRuntimeProfile,
		func(o *runtimeconfig.Overrides, v *string) { o.Profile = v },
	),
	"environment":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.Environment = v }),
	"session_dir":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SessionDir = v }),
	"cost_dir":     stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.CostDir = v }),
	"agent_preset": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.AgentPreset = v }),
	"tool_preset":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.ToolPreset = v }),
	"max_tokens":   positiveIntOverrideField("max_tokens", func(o *runtimeconfig.Overrides, v *int) { o.MaxTokens = v }),
	"max_iterations": positiveIntOverrideField(
		"max_iterations",
		func(o *runtimeconfig.Overrides, v *int) { o.MaxIterations = v },
	),
	"temperature": floatOverrideField("temperature", func(o *runtimeconfig.Overrides, v *float64) { o.Temperature = v }),
	"top_p":       floatOverrideField("top_p", func(o *runtimeconfig.Overrides, v *float64) { o.TopP = v }),
	"verbose":     boolOverrideField("verbose", func(o *runtimeconfig.Overrides, v *bool) { o.Verbose = v }),
	"disable_tui": boolOverrideField("disable_tui", func(o *runtimeconfig.Overrides, v *bool) { o.DisableTUI = v }),
	"follow_transcript": boolOverrideField(
		"follow_transcript",
		func(o *runtimeconfig.Overrides, v *bool) { o.FollowTranscript = v },
	),
	"follow_stream":  boolOverrideField("follow_stream", func(o *runtimeconfig.Overrides, v *bool) { o.FollowStream = v }),
	"stop_sequences": stopSequencesOverrideField(),
}

func setOverrideField(overrides *runtimeconfig.Overrides, key, value string) error {
	if overrides == nil {
		return fmt.Errorf("overrides not initialized")
	}
	normalizedKey := normalizeOverrideKey(key)
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return fmt.Errorf("value for %s cannot be empty", normalizedKey)
	}
	handler, ok := overrideFieldHandlers[normalizedKey]
	if !ok {
		return fmt.Errorf("unsupported override field %q", normalizedKey)
	}
	return handler.set(overrides, trimmedValue)
}

func clearOverrideField(overrides *runtimeconfig.Overrides, key, _ string) error {
	if overrides == nil {
		return fmt.Errorf("overrides not initialized")
	}
	normalizedKey := normalizeOverrideKey(key)
	handler, ok := overrideFieldHandlers[normalizedKey]
	if !ok {
		return fmt.Errorf("unsupported override field %q", normalizedKey)
	}
	handler.clear(overrides)
	return nil
}

func stringOverrideField(assign func(*runtimeconfig.Overrides, *string)) overrideFieldHandler {
	return normalizedStringOverrideField(func(value string) string { return value }, assign)
}

func normalizedStringOverrideField(
	normalize func(string) string,
	assign func(*runtimeconfig.Overrides, *string),
) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			assign(overrides, stringPtr(normalize(value)))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func positiveIntOverrideField(name string, assign func(*runtimeconfig.Overrides, *int)) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			parsed, err := parsePositiveInt(value, name)
			if err != nil {
				return err
			}
			assign(overrides, intPtr(parsed))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func floatOverrideField(name string, assign func(*runtimeconfig.Overrides, *float64)) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			parsed, err := parseFloat(value, name)
			if err != nil {
				return err
			}
			assign(overrides, floatPtr(parsed))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func boolOverrideField(name string, assign func(*runtimeconfig.Overrides, *bool)) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			parsed, err := parseBool(value, name)
			if err != nil {
				return err
			}
			assign(overrides, boolPtr(parsed))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func stopSequencesOverrideField() overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			seqs := splitListValue(value)
			if len(seqs) == 0 {
				return fmt.Errorf("stop_sequences requires at least one entry")
			}
			overrides.StopSequences = &seqs
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			overrides.StopSequences = nil
		},
	}
}

func parsePositiveInt(value string, name string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return parsed, nil
}

func parseFloat(value string, name string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a float: %w", name, err)
	}
	return parsed, nil
}

func parseBool(value string, name string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func normalizeOverrideKey(key string) string {
	trimmed := strings.TrimSpace(strings.ToLower(key))
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	return trimmed
}

func splitListValue(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	var result []string
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func stringPtr(value string) *string {
	v := value
	return &v
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func intPtr(value int) *int {
	v := value
	return &v
}

func floatPtr(value float64) *float64 {
	v := value
	return &v
}
