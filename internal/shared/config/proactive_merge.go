package config

import (
	"strings"

	"alex/internal/shared/utils"
)

func applyProactiveFileConfig(cfg *RuntimeConfig, meta *Metadata, file *ProactiveFileConfig) {
	if cfg == nil || file == nil {
		return
	}

	mergeProactiveConfig(&cfg.Proactive, file)
	if meta != nil {
		meta.sources["proactive"] = SourceFile
	}
}

func mergeProactiveConfig(target *ProactiveConfig, file *ProactiveFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if file.Prompt != nil {
		mergePromptConfig(&target.Prompt, file.Prompt)
	}
	if file.Memory != nil {
		mergeMemoryConfig(&target.Memory, file.Memory)
	}
	if file.Skills != nil {
		mergeSkillsConfig(&target.Skills, file.Skills)
	}
	if file.OKR != nil {
		mergeOKRConfig(&target.OKR, file.OKR)
	}
	if file.Scheduler != nil {
		mergeSchedulerConfig(&target.Scheduler, file.Scheduler)
	}
	if file.Timer != nil {
		mergeTimerConfig(&target.Timer, file.Timer)
	}
	if file.Attention != nil {
		mergeAttentionConfig(&target.Attention, file.Attention)
	}
}

func mergePromptConfig(target *PromptConfig, file *PromptFileConfig) {
	if target == nil || file == nil {
		return
	}
	if utils.HasContent(file.Mode) {
		target.Mode = strings.TrimSpace(file.Mode)
	}
	if utils.HasContent(file.Timezone) {
		target.Timezone = strings.TrimSpace(file.Timezone)
	}
	if file.BootstrapMaxChars != nil {
		target.BootstrapMaxChars = *file.BootstrapMaxChars
	}
	if len(file.BootstrapFiles) > 0 {
		files := make([]string, 0, len(file.BootstrapFiles))
		for _, path := range file.BootstrapFiles {
			if trimmed := strings.TrimSpace(path); trimmed != "" {
				files = append(files, trimmed)
			}
		}
		target.BootstrapFiles = files
	}
	if file.ReplyTagsEnabled != nil {
		target.ReplyTagsEnabled = *file.ReplyTagsEnabled
	}
}

func mergeMemoryConfig(target *MemoryConfig, file *MemoryFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if file.Index != nil {
		mergeMemoryIndexConfig(&target.Index, file.Index)
	}
	if file.ArchiveAfterDays != nil {
		target.ArchiveAfterDays = *file.ArchiveAfterDays
	}
	if utils.HasContent(file.CleanupInterval) {
		target.CleanupInterval = strings.TrimSpace(file.CleanupInterval)
	}
}

func mergeMemoryIndexConfig(target *MemoryIndexConfig, file *MemoryIndexFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if utils.HasContent(file.DBPath) {
		target.DBPath = strings.TrimSpace(file.DBPath)
	}
	if file.ChunkTokens != nil {
		target.ChunkTokens = *file.ChunkTokens
	}
	if file.ChunkOverlap != nil {
		target.ChunkOverlap = *file.ChunkOverlap
	}
	if file.MinScore != nil {
		target.MinScore = *file.MinScore
	}
	if file.FusionWeightVector != nil {
		target.FusionWeightVector = *file.FusionWeightVector
	}
	if file.FusionWeightBM25 != nil {
		target.FusionWeightBM25 = *file.FusionWeightBM25
	}
	if utils.HasContent(file.EmbedderModel) {
		target.EmbedderModel = strings.TrimSpace(file.EmbedderModel)
	}
}

func mergeSkillsConfig(target *SkillsConfig, file *SkillsFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.CacheTTLSeconds != nil {
		target.CacheTTLSeconds = *file.CacheTTLSeconds
	}
	if file.MetaOrchestratorEnabled != nil {
		target.MetaOrchestratorEnabled = *file.MetaOrchestratorEnabled
	}
	if file.SoulAutoEvolutionEnabled != nil {
		target.SoulAutoEvolutionEnabled = *file.SoulAutoEvolutionEnabled
	}
	if utils.HasContent(file.ProactiveLevel) {
		target.ProactiveLevel = strings.TrimSpace(file.ProactiveLevel)
	}
	if utils.HasContent(file.PolicyPath) {
		target.PolicyPath = strings.TrimSpace(file.PolicyPath)
	}
	if file.AutoActivation != nil {
		mergeSkillsAutoActivationConfig(&target.AutoActivation, file.AutoActivation)
	}
	if file.Feedback != nil {
		mergeSkillsFeedbackConfig(&target.Feedback, file.Feedback)
	}
}

func mergeSkillsAutoActivationConfig(target *SkillsAutoActivationConfig, file *SkillsAutoActivationFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if file.MaxActivated != nil {
		target.MaxActivated = *file.MaxActivated
	}
	if file.TokenBudget != nil {
		target.TokenBudget = *file.TokenBudget
	}
	if file.ConfidenceThreshold != nil {
		target.ConfidenceThreshold = *file.ConfidenceThreshold
	}
}

func mergeSkillsFeedbackConfig(target *SkillsFeedbackConfig, file *SkillsFeedbackFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if utils.HasContent(file.StorePath) {
		target.StorePath = strings.TrimSpace(file.StorePath)
	}
}

func mergeSchedulerConfig(target *SchedulerConfig, file *SchedulerFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if file.TriggerTimeoutSeconds != nil {
		target.TriggerTimeoutSeconds = *file.TriggerTimeoutSeconds
	}
	if utils.HasContent(file.ConcurrencyPolicy) {
		target.ConcurrencyPolicy = strings.TrimSpace(file.ConcurrencyPolicy)
	}
	if file.LeaderLockEnabled != nil {
		target.LeaderLockEnabled = *file.LeaderLockEnabled
	}
	if utils.HasContent(file.LeaderLockName) {
		target.LeaderLockName = strings.TrimSpace(file.LeaderLockName)
	}
	if file.LeaderLockAcquireIntervalSeconds != nil {
		target.LeaderLockAcquireIntervalSeconds = *file.LeaderLockAcquireIntervalSeconds
	}
	if utils.HasContent(file.JobStorePath) {
		target.JobStorePath = strings.TrimSpace(file.JobStorePath)
	}
	if file.CooldownSeconds != nil {
		target.CooldownSeconds = *file.CooldownSeconds
	}
	if file.MaxConcurrent != nil {
		target.MaxConcurrent = *file.MaxConcurrent
	}
	if file.RecoveryMaxRetries != nil {
		target.RecoveryMaxRetries = *file.RecoveryMaxRetries
	}
	if file.RecoveryBackoffSeconds != nil {
		target.RecoveryBackoffSeconds = *file.RecoveryBackoffSeconds
	}
	if file.CalendarReminder != nil {
		mergeCalendarReminderConfig(&target.CalendarReminder, file.CalendarReminder)
	}
	if file.Heartbeat != nil {
		mergeHeartbeatConfig(&target.Heartbeat, file.Heartbeat)
	}
	if file.MilestoneCheckin != nil {
		mergeMilestoneCheckinConfig(&target.MilestoneCheckin, file.MilestoneCheckin)
	}
	if len(file.Triggers) > 0 {
		triggers := make([]SchedulerTriggerConfig, 0, len(file.Triggers))
		for _, trigger := range file.Triggers {
			cfg := SchedulerTriggerConfig{
				Name:     strings.TrimSpace(trigger.Name),
				Schedule: strings.TrimSpace(trigger.Schedule),
				Task:     strings.TrimSpace(trigger.Task),
				Channel:  strings.TrimSpace(trigger.Channel),
				UserID:   strings.TrimSpace(trigger.UserID),
				ChatID:   strings.TrimSpace(trigger.ChatID),
				Risk:     strings.TrimSpace(trigger.Risk),
			}
			if trigger.ApprovalRequired != nil {
				cfg.ApprovalRequired = *trigger.ApprovalRequired
			}
			triggers = append(triggers, cfg)
		}
		target.Triggers = triggers
	}
}

func mergeHeartbeatConfig(target *HeartbeatConfig, file *HeartbeatFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if utils.HasContent(file.Schedule) {
		target.Schedule = strings.TrimSpace(file.Schedule)
	}
	if utils.HasContent(file.Task) {
		target.Task = strings.TrimSpace(file.Task)
	}
	if utils.HasContent(file.Channel) {
		target.Channel = strings.TrimSpace(file.Channel)
	}
	if utils.HasContent(file.UserID) {
		target.UserID = strings.TrimSpace(file.UserID)
	}
	if utils.HasContent(file.ChatID) {
		target.ChatID = strings.TrimSpace(file.ChatID)
	}
	if len(file.QuietHours) == 2 {
		target.QuietHours = [2]int{file.QuietHours[0], file.QuietHours[1]}
	}
	if file.WindowLookbackHr != nil {
		target.WindowLookbackHr = *file.WindowLookbackHr
	}
}

func mergeTimerConfig(target *TimerConfig, file *TimerFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if utils.HasContent(file.StorePath) {
		target.StorePath = strings.TrimSpace(file.StorePath)
	}
	if file.MaxTimers != nil {
		target.MaxTimers = *file.MaxTimers
	}
	if file.TaskTimeoutSeconds != nil {
		target.TaskTimeoutSeconds = *file.TaskTimeoutSeconds
	}
	if file.HeartbeatEnabled != nil {
		target.HeartbeatEnabled = *file.HeartbeatEnabled
	}
	if file.HeartbeatMinutes != nil {
		target.HeartbeatMinutes = *file.HeartbeatMinutes
	}
}

func mergeCalendarReminderConfig(target *CalendarReminderConfig, file *CalendarReminderFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if utils.HasContent(file.Schedule) {
		target.Schedule = strings.TrimSpace(file.Schedule)
	}
	if file.LookAheadMinutes != nil {
		target.LookAheadMinutes = *file.LookAheadMinutes
	}
	if utils.HasContent(file.Channel) {
		target.Channel = strings.TrimSpace(file.Channel)
	}
	if utils.HasContent(file.UserID) {
		target.UserID = strings.TrimSpace(file.UserID)
	}
	if utils.HasContent(file.ChatID) {
		target.ChatID = strings.TrimSpace(file.ChatID)
	}
}

func mergeMilestoneCheckinConfig(target *MilestoneCheckinConfig, file *MilestoneCheckinFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if utils.HasContent(file.Schedule) {
		target.Schedule = strings.TrimSpace(file.Schedule)
	}
	if file.LookbackSeconds != nil {
		target.LookbackSeconds = *file.LookbackSeconds
	}
	if utils.HasContent(file.Channel) {
		target.Channel = strings.TrimSpace(file.Channel)
	}
	if utils.HasContent(file.ChatID) {
		target.ChatID = strings.TrimSpace(file.ChatID)
	}
	if file.IncludeActive != nil {
		target.IncludeActive = *file.IncludeActive
	}
	if file.IncludeCompleted != nil {
		target.IncludeCompleted = *file.IncludeCompleted
	}
}

func mergeAttentionConfig(target *AttentionConfig, file *AttentionFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.MaxDailyNotifications != nil {
		target.MaxDailyNotifications = *file.MaxDailyNotifications
	}
	if file.MinIntervalSeconds != nil {
		target.MinIntervalSeconds = *file.MinIntervalSeconds
	}
	if file.PriorityThreshold != nil {
		target.PriorityThreshold = *file.PriorityThreshold
	}
	if len(file.QuietHours) == 2 {
		target.QuietHours = [2]int{file.QuietHours[0], file.QuietHours[1]}
	}
}

func mergeOKRConfig(target *OKRProactiveConfig, file *OKRFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if utils.HasContent(file.GoalsRoot) {
		target.GoalsRoot = strings.TrimSpace(file.GoalsRoot)
	}
	if file.AutoInject != nil {
		target.AutoInject = *file.AutoInject
	}
}

func expandProactiveFileConfigEnv(lookup EnvLookup, file *ProactiveFileConfig) {
	if file == nil {
		return
	}
	if file.Skills != nil {
		if file.Skills.Feedback != nil {
			file.Skills.Feedback.StorePath = expandEnvValue(lookup, file.Skills.Feedback.StorePath)
		}
	}
	if file.Memory != nil && file.Memory.Index != nil {
		file.Memory.Index.DBPath = expandEnvValue(lookup, file.Memory.Index.DBPath)
		file.Memory.Index.EmbedderModel = expandEnvValue(lookup, file.Memory.Index.EmbedderModel)
	}
	if file.OKR != nil {
		file.OKR.GoalsRoot = expandEnvValue(lookup, file.OKR.GoalsRoot)
	}
	if file.Prompt != nil {
		file.Prompt.Mode = expandEnvValue(lookup, file.Prompt.Mode)
		file.Prompt.Timezone = expandEnvValue(lookup, file.Prompt.Timezone)
		if len(file.Prompt.BootstrapFiles) > 0 {
			expanded := make([]string, 0, len(file.Prompt.BootstrapFiles))
			for _, path := range file.Prompt.BootstrapFiles {
				expanded = append(expanded, expandEnvValue(lookup, path))
			}
			file.Prompt.BootstrapFiles = expanded
		}
	}
	if file.Scheduler != nil {
		file.Scheduler.LeaderLockName = expandEnvValue(lookup, file.Scheduler.LeaderLockName)
		for i := range file.Scheduler.Triggers {
			file.Scheduler.Triggers[i].Task = expandEnvValue(lookup, file.Scheduler.Triggers[i].Task)
			file.Scheduler.Triggers[i].UserID = expandEnvValue(lookup, file.Scheduler.Triggers[i].UserID)
			file.Scheduler.Triggers[i].ChatID = expandEnvValue(lookup, file.Scheduler.Triggers[i].ChatID)
		}
		if file.Scheduler.Heartbeat != nil {
			file.Scheduler.Heartbeat.Schedule = expandEnvValue(lookup, file.Scheduler.Heartbeat.Schedule)
			file.Scheduler.Heartbeat.Task = expandEnvValue(lookup, file.Scheduler.Heartbeat.Task)
			file.Scheduler.Heartbeat.Channel = expandEnvValue(lookup, file.Scheduler.Heartbeat.Channel)
			file.Scheduler.Heartbeat.UserID = expandEnvValue(lookup, file.Scheduler.Heartbeat.UserID)
			file.Scheduler.Heartbeat.ChatID = expandEnvValue(lookup, file.Scheduler.Heartbeat.ChatID)
		}
		if file.Scheduler.CalendarReminder != nil {
			file.Scheduler.CalendarReminder.Schedule = expandEnvValue(lookup, file.Scheduler.CalendarReminder.Schedule)
			file.Scheduler.CalendarReminder.Channel = expandEnvValue(lookup, file.Scheduler.CalendarReminder.Channel)
			file.Scheduler.CalendarReminder.UserID = expandEnvValue(lookup, file.Scheduler.CalendarReminder.UserID)
			file.Scheduler.CalendarReminder.ChatID = expandEnvValue(lookup, file.Scheduler.CalendarReminder.ChatID)
		}
		if file.Scheduler.MilestoneCheckin != nil {
			file.Scheduler.MilestoneCheckin.Schedule = expandEnvValue(lookup, file.Scheduler.MilestoneCheckin.Schedule)
			file.Scheduler.MilestoneCheckin.Channel = expandEnvValue(lookup, file.Scheduler.MilestoneCheckin.Channel)
			file.Scheduler.MilestoneCheckin.ChatID = expandEnvValue(lookup, file.Scheduler.MilestoneCheckin.ChatID)
		}
	}
	if file.Timer != nil {
		file.Timer.StorePath = expandEnvValue(lookup, file.Timer.StorePath)
	}
}
