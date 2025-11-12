package analytics

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

type trackingPlan struct {
	Events []struct {
		Event string `yaml:"event"`
	} `yaml:"events"`
}

func TestTrackingPlanMatchesImplementedEvents(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to determine working directory: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))

	planPath := filepath.Join(repoRoot, "docs", "analytics", "tracking-plan.yaml")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("failed to read tracking plan: %v", err)
	}

	var plan trackingPlan
	if err := yaml.Unmarshal(planData, &plan); err != nil {
		t.Fatalf("failed to parse tracking plan: %v", err)
	}

	planEvents := make(map[string]struct{}, len(plan.Events))
	for _, evt := range plan.Events {
		if evt.Event == "" {
			continue
		}
		planEvents[evt.Event] = struct{}{}
	}

	if len(planEvents) == 0 {
		t.Fatalf("tracking plan did not contain any events")
	}

	frontendEvents := parseFrontendEvents(t, filepath.Join(repoRoot, "web", "lib", "analytics", "events.ts"))

	serverEvents := []string{
		EventTaskSubmitted,
		EventTaskSubmissionFailed,
		EventTaskRetriedWithoutSession,
		EventTaskCancelRequested,
		EventTaskCancelFailed,
		EventTaskExecutionStarted,
		EventTaskExecutionCompleted,
		EventTaskExecutionFailed,
		EventTaskExecutionCancelled,
	}

	for _, event := range serverEvents {
		if _, ok := planEvents[event]; !ok {
			t.Errorf("server analytics event %q missing from tracking plan", event)
		}
	}

	for event := range frontendEvents {
		if _, ok := planEvents[event]; !ok {
			t.Errorf("frontend analytics event %q missing from tracking plan", event)
		}
	}

	usedEvents := make(map[string]struct{}, len(serverEvents)+len(frontendEvents))
	for _, event := range serverEvents {
		usedEvents[event] = struct{}{}
	}
	for event := range frontendEvents {
		usedEvents[event] = struct{}{}
	}

	var unused []string
	for event := range planEvents {
		if _, ok := usedEvents[event]; !ok {
			unused = append(unused, event)
		}
	}

	if len(unused) > 0 {
		sort.Strings(unused)
		t.Errorf("tracking plan contains events without implementations: %v", unused)
	}
}

func parseFrontendEvents(t *testing.T, path string) map[string]struct{} {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read frontend analytics events: %v", err)
	}

	pattern := regexp.MustCompile(`:\s*'([a-z0-9_]+)'`)
	matches := pattern.FindAllStringSubmatch(string(data), -1)
	events := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		events[match[1]] = struct{}{}
	}

	if len(events) == 0 {
		t.Fatalf("no analytics events could be parsed from %s", path)
	}

	return events
}
