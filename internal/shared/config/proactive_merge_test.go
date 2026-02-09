package config

import "testing"

func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }

func TestMergeOKRConfig(t *testing.T) {
	target := OKRProactiveConfig{
		Enabled:    false,
		GoalsRoot:  "/default/goals",
		AutoInject: true,
	}

	file := &OKRFileConfig{
		Enabled:    boolPtr(true),
		GoalsRoot:  "/custom/goals",
		AutoInject: boolPtr(false),
	}

	mergeOKRConfig(&target, file)

	if !target.Enabled {
		t.Error("expected Enabled to be true")
	}
	if target.GoalsRoot != "/custom/goals" {
		t.Errorf("expected GoalsRoot '/custom/goals', got %q", target.GoalsRoot)
	}
	if target.AutoInject {
		t.Error("expected AutoInject to be false")
	}
}

func TestMergeOKRConfig_PartialOverride(t *testing.T) {
	target := OKRProactiveConfig{
		Enabled:    true,
		GoalsRoot:  "/default/goals",
		AutoInject: true,
	}

	// Only override Enabled
	file := &OKRFileConfig{
		Enabled: boolPtr(false),
	}

	mergeOKRConfig(&target, file)

	if target.Enabled {
		t.Error("expected Enabled to be false")
	}
	if target.GoalsRoot != "/default/goals" {
		t.Errorf("expected GoalsRoot unchanged, got %q", target.GoalsRoot)
	}
	if !target.AutoInject {
		t.Error("expected AutoInject unchanged (true)")
	}
}

func TestMergeOKRConfig_NilInputs(t *testing.T) {
	// Should not panic
	mergeOKRConfig(nil, nil)
	mergeOKRConfig(nil, &OKRFileConfig{})

	target := OKRProactiveConfig{Enabled: true}
	mergeOKRConfig(&target, nil)
	if !target.Enabled {
		t.Error("nil file should not change target")
	}
}

func TestMergeProactiveConfig_IncludesOKR(t *testing.T) {
	target := DefaultProactiveConfig()
	file := &ProactiveFileConfig{
		OKR: &OKRFileConfig{
			Enabled:   boolPtr(false),
			GoalsRoot: "/test/goals",
		},
	}

	mergeProactiveConfig(&target, file)

	if target.OKR.Enabled {
		t.Error("expected OKR.Enabled to be overridden to false")
	}
	if target.OKR.GoalsRoot != "/test/goals" {
		t.Errorf("expected OKR.GoalsRoot '/test/goals', got %q", target.OKR.GoalsRoot)
	}
}

func TestExpandProactiveFileConfigEnv_OKR(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "GOALS_DIR" {
			return "/from/env/goals", true
		}
		return "", false
	}

	file := &ProactiveFileConfig{
		OKR: &OKRFileConfig{
			GoalsRoot: "${GOALS_DIR}",
		},
	}

	expandProactiveFileConfigEnv(lookup, file)

	if file.OKR.GoalsRoot != "/from/env/goals" {
		t.Errorf("expected expanded GoalsRoot '/from/env/goals', got %q", file.OKR.GoalsRoot)
	}
}

func TestExpandProactiveFileConfigEnv_MemoryIndex(t *testing.T) {
	lookup := func(key string) (string, bool) {
		switch key {
		case "MEMORY_INDEX_DB":
			return "/from/env/index.sqlite", true
		case "EMBED_MODEL":
			return "nomic-embed-text", true
		default:
			return "", false
		}
	}

	file := &ProactiveFileConfig{
		Memory: &MemoryFileConfig{
			Index: &MemoryIndexFileConfig{
				DBPath:        "${MEMORY_INDEX_DB}",
				EmbedderModel: "${EMBED_MODEL}",
			},
		},
	}

	expandProactiveFileConfigEnv(lookup, file)

	if file.Memory.Index.DBPath != "/from/env/index.sqlite" {
		t.Errorf("expected expanded DBPath '/from/env/index.sqlite', got %q", file.Memory.Index.DBPath)
	}
	if file.Memory.Index.EmbedderModel != "nomic-embed-text" {
		t.Errorf("expected expanded EmbedderModel 'nomic-embed-text', got %q", file.Memory.Index.EmbedderModel)
	}
}

func TestMergeFinalAnswerReviewConfig(t *testing.T) {
	target := FinalAnswerReviewConfig{
		Enabled:            false,
		MaxExtraIterations: 1,
	}
	file := &FinalAnswerReviewFileConfig{
		Enabled:            boolPtr(true),
		MaxExtraIterations: intPtr(2),
	}

	mergeFinalAnswerReviewConfig(&target, file)

	if !target.Enabled {
		t.Error("expected Enabled to be true")
	}
	if target.MaxExtraIterations != 2 {
		t.Errorf("expected MaxExtraIterations=2, got %d", target.MaxExtraIterations)
	}
}

func TestMergeProactiveConfig_IncludesFinalAnswerReview(t *testing.T) {
	target := DefaultProactiveConfig()
	file := &ProactiveFileConfig{
		FinalAnswerReview: &FinalAnswerReviewFileConfig{
			Enabled:            boolPtr(false),
			MaxExtraIterations: intPtr(3),
		},
	}

	mergeProactiveConfig(&target, file)

	if target.FinalAnswerReview.Enabled {
		t.Error("expected FinalAnswerReview.Enabled to be overridden to false")
	}
	if target.FinalAnswerReview.MaxExtraIterations != 3 {
		t.Errorf("expected FinalAnswerReview.MaxExtraIterations=3, got %d", target.FinalAnswerReview.MaxExtraIterations)
	}
}

func TestMergeSchedulerConfig_IncludesChatIDAndCalendarReminder(t *testing.T) {
	target := DefaultProactiveConfig().Scheduler
	target.CalendarReminder = CalendarReminderConfig{
		Enabled:          false,
		Schedule:         "*/30 * * * *",
		LookAheadMinutes: 15,
		Channel:          "lark",
		UserID:           "ou_default",
		ChatID:           "oc_default",
	}

	file := &SchedulerFileConfig{
		Triggers: []SchedulerTriggerFileConfig{
			{
				Name:             "standup",
				Schedule:         "0 9 * * *",
				Task:             "send standup",
				Channel:          "lark",
				UserID:           "ou_trigger",
				ChatID:           "oc_trigger",
				ApprovalRequired: boolPtr(true),
				Risk:             "medium",
			},
		},
		CalendarReminder: &CalendarReminderFileConfig{
			Enabled:          boolPtr(true),
			Schedule:         "*/5 * * * *",
			LookAheadMinutes: intPtr(45),
			Channel:          "lark",
			UserID:           "ou_calendar",
			ChatID:           "oc_calendar",
		},
	}

	mergeSchedulerConfig(&target, file)

	if len(target.Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(target.Triggers))
	}
	trigger := target.Triggers[0]
	if trigger.ChatID != "oc_trigger" {
		t.Fatalf("expected trigger chat_id to merge, got %q", trigger.ChatID)
	}
	if !trigger.ApprovalRequired {
		t.Fatalf("expected trigger approval_required=true")
	}

	if !target.CalendarReminder.Enabled {
		t.Fatalf("expected calendar reminder enabled")
	}
	if target.CalendarReminder.Schedule != "*/5 * * * *" {
		t.Fatalf("expected calendar schedule merge, got %q", target.CalendarReminder.Schedule)
	}
	if target.CalendarReminder.LookAheadMinutes != 45 {
		t.Fatalf("expected look_ahead_minutes=45, got %d", target.CalendarReminder.LookAheadMinutes)
	}
	if target.CalendarReminder.Channel != "lark" {
		t.Fatalf("expected calendar channel merge, got %q", target.CalendarReminder.Channel)
	}
	if target.CalendarReminder.UserID != "ou_calendar" {
		t.Fatalf("expected calendar user_id merge, got %q", target.CalendarReminder.UserID)
	}
	if target.CalendarReminder.ChatID != "oc_calendar" {
		t.Fatalf("expected calendar chat_id merge, got %q", target.CalendarReminder.ChatID)
	}
}

func TestExpandProactiveFileConfigEnv_SchedulerTriggerAndCalendar(t *testing.T) {
	lookup := func(key string) (string, bool) {
		switch key {
		case "TRIGGER_TASK":
			return "expanded task", true
		case "TRIGGER_USER":
			return "ou_trigger", true
		case "TRIGGER_CHAT":
			return "oc_trigger", true
		case "CALENDAR_SCHEDULE":
			return "*/10 * * * *", true
		case "CALENDAR_CHANNEL":
			return "lark", true
		case "CALENDAR_USER":
			return "ou_calendar", true
		case "CALENDAR_CHAT":
			return "oc_calendar", true
		default:
			return "", false
		}
	}

	file := &ProactiveFileConfig{
		Scheduler: &SchedulerFileConfig{
			Triggers: []SchedulerTriggerFileConfig{
				{
					Name:     "expanded",
					Schedule: "0 9 * * *",
					Task:     "${TRIGGER_TASK}",
					UserID:   "${TRIGGER_USER}",
					ChatID:   "${TRIGGER_CHAT}",
				},
			},
			CalendarReminder: &CalendarReminderFileConfig{
				Schedule: "${CALENDAR_SCHEDULE}",
				Channel:  "${CALENDAR_CHANNEL}",
				UserID:   "${CALENDAR_USER}",
				ChatID:   "${CALENDAR_CHAT}",
			},
		},
	}

	expandProactiveFileConfigEnv(lookup, file)

	if got := file.Scheduler.Triggers[0].Task; got != "expanded task" {
		t.Fatalf("expected expanded trigger task, got %q", got)
	}
	if got := file.Scheduler.Triggers[0].UserID; got != "ou_trigger" {
		t.Fatalf("expected expanded trigger user_id, got %q", got)
	}
	if got := file.Scheduler.Triggers[0].ChatID; got != "oc_trigger" {
		t.Fatalf("expected expanded trigger chat_id, got %q", got)
	}
	if got := file.Scheduler.CalendarReminder.Schedule; got != "*/10 * * * *" {
		t.Fatalf("expected expanded calendar schedule, got %q", got)
	}
	if got := file.Scheduler.CalendarReminder.Channel; got != "lark" {
		t.Fatalf("expected expanded calendar channel, got %q", got)
	}
	if got := file.Scheduler.CalendarReminder.UserID; got != "ou_calendar" {
		t.Fatalf("expected expanded calendar user_id, got %q", got)
	}
	if got := file.Scheduler.CalendarReminder.ChatID; got != "oc_calendar" {
		t.Fatalf("expected expanded calendar chat_id, got %q", got)
	}
}
