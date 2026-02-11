package config

import "strings"

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
	if file.FinalAnswerReview != nil {
		mergeFinalAnswerReviewConfig(&target.FinalAnswerReview, file.FinalAnswerReview)
	}
	if file.Attention != nil {
		mergeAttentionConfig(&target.Attention, file.Attention)
	}
	if file.Kernel != nil {
		mergeKernelConfig(&target.Kernel, file.Kernel)
	}
}

func mergePromptConfig(target *PromptConfig, file *PromptFileConfig) {
	if target == nil || file == nil {
		return
	}
	if strings.TrimSpace(file.Mode) != "" {
		target.Mode = strings.TrimSpace(file.Mode)
	}
	if strings.TrimSpace(file.Timezone) != "" {
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
}

func mergeMemoryIndexConfig(target *MemoryIndexConfig, file *MemoryIndexFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if strings.TrimSpace(file.DBPath) != "" {
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
	if strings.TrimSpace(file.EmbedderModel) != "" {
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
	if strings.TrimSpace(file.StorePath) != "" {
		target.StorePath = strings.TrimSpace(file.StorePath)
	}
}

func mergeFinalAnswerReviewConfig(target *FinalAnswerReviewConfig, file *FinalAnswerReviewFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if file.MaxExtraIterations != nil {
		target.MaxExtraIterations = *file.MaxExtraIterations
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
	if strings.TrimSpace(file.ConcurrencyPolicy) != "" {
		target.ConcurrencyPolicy = strings.TrimSpace(file.ConcurrencyPolicy)
	}
	if strings.TrimSpace(file.JobStorePath) != "" {
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
	if strings.TrimSpace(file.Schedule) != "" {
		target.Schedule = strings.TrimSpace(file.Schedule)
	}
	if strings.TrimSpace(file.Task) != "" {
		target.Task = strings.TrimSpace(file.Task)
	}
	if strings.TrimSpace(file.Channel) != "" {
		target.Channel = strings.TrimSpace(file.Channel)
	}
	if strings.TrimSpace(file.UserID) != "" {
		target.UserID = strings.TrimSpace(file.UserID)
	}
	if strings.TrimSpace(file.ChatID) != "" {
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
	if strings.TrimSpace(file.StorePath) != "" {
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
	if strings.TrimSpace(file.Schedule) != "" {
		target.Schedule = strings.TrimSpace(file.Schedule)
	}
	if file.LookAheadMinutes != nil {
		target.LookAheadMinutes = *file.LookAheadMinutes
	}
	if strings.TrimSpace(file.Channel) != "" {
		target.Channel = strings.TrimSpace(file.Channel)
	}
	if strings.TrimSpace(file.UserID) != "" {
		target.UserID = strings.TrimSpace(file.UserID)
	}
	if strings.TrimSpace(file.ChatID) != "" {
		target.ChatID = strings.TrimSpace(file.ChatID)
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
	if strings.TrimSpace(file.GoalsRoot) != "" {
		target.GoalsRoot = strings.TrimSpace(file.GoalsRoot)
	}
	if file.AutoInject != nil {
		target.AutoInject = *file.AutoInject
	}
}

func mergeKernelConfig(target *KernelProactiveConfig, file *KernelFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if strings.TrimSpace(file.KernelID) != "" {
		target.KernelID = strings.TrimSpace(file.KernelID)
	}
	if strings.TrimSpace(file.Schedule) != "" {
		target.Schedule = strings.TrimSpace(file.Schedule)
	}
	if strings.TrimSpace(file.StateDir) != "" {
		target.StateDir = strings.TrimSpace(file.StateDir)
	}
	if strings.TrimSpace(file.SeedState) != "" {
		target.SeedState = file.SeedState // preserve whitespace in seed state content
	}
	if file.TimeoutSeconds != nil {
		target.TimeoutSeconds = *file.TimeoutSeconds
	}
	if file.LeaseSeconds != nil {
		target.LeaseSeconds = *file.LeaseSeconds
	}
	if file.MaxConcurrent != nil {
		target.MaxConcurrent = *file.MaxConcurrent
	}
	if strings.TrimSpace(file.Channel) != "" {
		target.Channel = strings.TrimSpace(file.Channel)
	}
	if strings.TrimSpace(file.UserID) != "" {
		target.UserID = strings.TrimSpace(file.UserID)
	}
	if strings.TrimSpace(file.ChatID) != "" {
		target.ChatID = strings.TrimSpace(file.ChatID)
	}
	if len(file.Agents) > 0 {
		agents := make([]KernelAgentProactiveConfig, 0, len(file.Agents))
		for _, a := range file.Agents {
			enabled := true
			if a.Enabled != nil {
				enabled = *a.Enabled
			}
			priority := 5
			if a.Priority != nil {
				priority = *a.Priority
			}
			agents = append(agents, KernelAgentProactiveConfig{
				AgentID:  strings.TrimSpace(a.AgentID),
				Prompt:   a.Prompt,
				Priority: priority,
				Enabled:  enabled,
				Metadata: a.Metadata,
			})
		}
		target.Agents = agents
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
	}
	if file.Timer != nil {
		file.Timer.StorePath = expandEnvValue(lookup, file.Timer.StorePath)
	}
	if file.Kernel != nil {
		file.Kernel.StateDir = expandEnvValue(lookup, file.Kernel.StateDir)
		file.Kernel.Channel = expandEnvValue(lookup, file.Kernel.Channel)
		file.Kernel.UserID = expandEnvValue(lookup, file.Kernel.UserID)
		file.Kernel.ChatID = expandEnvValue(lookup, file.Kernel.ChatID)
		for i := range file.Kernel.Agents {
			file.Kernel.Agents[i].Prompt = expandEnvValue(lookup, file.Kernel.Agents[i].Prompt)
		}
	}
}
