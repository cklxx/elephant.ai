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
	if file.FinalAnswerReview != nil {
		mergeFinalAnswerReviewConfig(&target.FinalAnswerReview, file.FinalAnswerReview)
	}
	if file.Attention != nil {
		mergeAttentionConfig(&target.Attention, file.Attention)
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
	if len(file.Triggers) > 0 {
		triggers := make([]SchedulerTriggerConfig, 0, len(file.Triggers))
		for _, trigger := range file.Triggers {
			cfg := SchedulerTriggerConfig{
				Name:     strings.TrimSpace(trigger.Name),
				Schedule: strings.TrimSpace(trigger.Schedule),
				Task:     strings.TrimSpace(trigger.Task),
				Channel:  strings.TrimSpace(trigger.Channel),
				UserID:   strings.TrimSpace(trigger.UserID),
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
	if file.Scheduler != nil {
		for i := range file.Scheduler.Triggers {
			file.Scheduler.Triggers[i].Task = expandEnvValue(lookup, file.Scheduler.Triggers[i].Task)
		}
	}
}
