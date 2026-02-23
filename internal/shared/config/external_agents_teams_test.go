package config

import "testing"

func TestLoadFromFile_ExternalAgentTeams(t *testing.T) {
	fileData := []byte(`
runtime:
  api_key: "sk-test"
  external_agents:
    teams:
      - name: "execute_and_report"
        description: "Codex executes and Claude reports"
        roles:
          - name: "executor"
            agent_type: "codex"
            prompt_template: "Implement: {GOAL}"
            execution_mode: "execute"
            autonomy_level: "full"
            workspace_mode: "worktree"
            config:
              approval_policy: "never"
              sandbox: "danger-full-access"
              binary: "codex"
          - name: "reporter"
            agent_type: "claude_code"
            prompt_template: "Summarize: {GOAL}"
            execution_mode: "execute"
            autonomy_level: "full"
            workspace_mode: "shared"
            inherit_context: true
        stages:
          - name: "execution"
            roles: ["executor"]
          - name: "reporting"
            roles: ["reporter"]
`)

	cfg, meta, err := Load(
		WithEnv(envMap{}.Lookup),
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.ExternalAgents.Teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(cfg.ExternalAgents.Teams))
	}
	team := cfg.ExternalAgents.Teams[0]
	if team.Name != "execute_and_report" {
		t.Fatalf("expected team name execute_and_report, got %q", team.Name)
	}
	if len(team.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(team.Roles))
	}
	if len(team.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(team.Stages))
	}

	executor := team.Roles[0]
	if executor.AgentType != "codex" {
		t.Fatalf("expected first role agent_type codex, got %q", executor.AgentType)
	}
	if executor.WorkspaceMode != "worktree" {
		t.Fatalf("expected executor workspace_mode worktree, got %q", executor.WorkspaceMode)
	}
	if executor.Config["sandbox"] != "danger-full-access" {
		t.Fatalf("expected executor sandbox override, got %q", executor.Config["sandbox"])
	}
	if executor.InheritContext {
		t.Fatal("expected executor inherit_context to default false")
	}

	reporter := team.Roles[1]
	if !reporter.InheritContext {
		t.Fatal("expected reporter inherit_context=true")
	}
	if reporter.AgentType != "claude_code" {
		t.Fatalf("expected reporter agent_type claude_code, got %q", reporter.AgentType)
	}

	if got := meta.Source("external_agents.teams"); got != SourceFile {
		t.Fatalf("expected external_agents.teams source=file, got %s", got)
	}
}

func TestLoadFromFile_ExternalAgentTeamsEnvExpansion(t *testing.T) {
	fileData := []byte(`
runtime:
  api_key: "sk-test"
  external_agents:
    teams:
      - name: "${TEAM_NAME}"
        description: "${TEAM_DESC}"
        roles:
          - name: "${ROLE_NAME}"
            agent_type: "${ROLE_AGENT}"
            prompt_template: "${ROLE_PROMPT}"
            execution_mode: "${ROLE_EXECUTION_MODE}"
            autonomy_level: "${ROLE_AUTONOMY}"
            workspace_mode: "${ROLE_WORKSPACE}"
            config:
              sandbox: "${ROLE_SANDBOX}"
              approval_policy: "${ROLE_APPROVAL}"
        stages:
          - name: "${STAGE_NAME}"
            roles: ["${ROLE_NAME}"]
`)

	env := envMap{
		"TEAM_NAME":           "env_team",
		"TEAM_DESC":           "loaded from env",
		"ROLE_NAME":           "env_executor",
		"ROLE_AGENT":          "codex",
		"ROLE_PROMPT":         "Execute env goal: {GOAL}",
		"ROLE_EXECUTION_MODE": "plan",
		"ROLE_AUTONOMY":       "semi",
		"ROLE_WORKSPACE":      "worktree",
		"ROLE_SANDBOX":        "danger-full-access",
		"ROLE_APPROVAL":       "never",
		"STAGE_NAME":          "env_stage",
	}

	cfg, _, err := Load(
		WithEnv(env.Lookup),
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.ExternalAgents.Teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(cfg.ExternalAgents.Teams))
	}
	team := cfg.ExternalAgents.Teams[0]
	if team.Name != "env_team" {
		t.Fatalf("expected expanded team name env_team, got %q", team.Name)
	}
	if team.Description != "loaded from env" {
		t.Fatalf("expected expanded team description, got %q", team.Description)
	}
	if len(team.Roles) != 1 {
		t.Fatalf("expected 1 role, got %d", len(team.Roles))
	}
	role := team.Roles[0]
	if role.Name != "env_executor" {
		t.Fatalf("expected expanded role name env_executor, got %q", role.Name)
	}
	if role.AgentType != "codex" {
		t.Fatalf("expected expanded role agent codex, got %q", role.AgentType)
	}
	if role.ExecutionMode != "plan" {
		t.Fatalf("expected expanded execution_mode plan, got %q", role.ExecutionMode)
	}
	if role.AutonomyLevel != "semi" {
		t.Fatalf("expected expanded autonomy_level semi, got %q", role.AutonomyLevel)
	}
	if role.WorkspaceMode != "worktree" {
		t.Fatalf("expected expanded workspace_mode worktree, got %q", role.WorkspaceMode)
	}
	if role.Config["sandbox"] != "danger-full-access" {
		t.Fatalf("expected expanded sandbox config, got %q", role.Config["sandbox"])
	}
	if role.Config["approval_policy"] != "never" {
		t.Fatalf("expected expanded approval_policy config, got %q", role.Config["approval_policy"])
	}
	if len(team.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(team.Stages))
	}
	if team.Stages[0].Name != "env_stage" {
		t.Fatalf("expected expanded stage name env_stage, got %q", team.Stages[0].Name)
	}
	if len(team.Stages[0].Roles) != 1 || team.Stages[0].Roles[0] != "env_executor" {
		t.Fatalf("expected expanded stage role env_executor, got %#v", team.Stages[0].Roles)
	}
}
