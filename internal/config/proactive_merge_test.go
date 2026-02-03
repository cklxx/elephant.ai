package config

import "testing"

func boolPtr(v bool) *bool { return &v }

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
