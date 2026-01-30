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
	if file.RAG != nil {
		mergeRAGConfig(&target.RAG, file.RAG)
	}
	if file.Scheduler != nil {
		mergeSchedulerConfig(&target.Scheduler, file.Scheduler)
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
	if file.AutoRecall != nil {
		target.AutoRecall = *file.AutoRecall
	}
	if file.AutoCapture != nil {
		target.AutoCapture = *file.AutoCapture
	}
	if file.CaptureMessages != nil {
		target.CaptureMessages = *file.CaptureMessages
	}
	if file.MaxRecalls != nil {
		target.MaxRecalls = *file.MaxRecalls
	}
	if file.RefreshInterval != nil {
		target.RefreshInterval = *file.RefreshInterval
	}
	if file.MaxRefreshTokens != nil {
		target.MaxRefreshTokens = *file.MaxRefreshTokens
	}
	if strings.TrimSpace(file.Store) != "" {
		target.Store = strings.TrimSpace(file.Store)
	}
	if file.DedupeThreshold != nil {
		target.DedupeThreshold = *file.DedupeThreshold
	}
	if file.Hybrid != nil {
		mergeMemoryHybridConfig(&target.Hybrid, file.Hybrid)
	}
}

func mergeMemoryHybridConfig(target *MemoryHybridConfig, file *MemoryHybridFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Alpha != nil {
		target.Alpha = *file.Alpha
	}
	if file.MinSimilarity != nil {
		target.MinSimilarity = *file.MinSimilarity
	}
	if strings.TrimSpace(file.PersistDir) != "" {
		target.PersistDir = strings.TrimSpace(file.PersistDir)
	}
	if strings.TrimSpace(file.Collection) != "" {
		target.Collection = strings.TrimSpace(file.Collection)
	}
	if strings.TrimSpace(file.EmbedderModel) != "" {
		target.EmbedderModel = strings.TrimSpace(file.EmbedderModel)
	}
	if strings.TrimSpace(file.EmbedderBaseURL) != "" {
		target.EmbedderBaseURL = strings.TrimSpace(file.EmbedderBaseURL)
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

func mergeRAGConfig(target *RAGConfig, file *RAGFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
	}
	if strings.TrimSpace(file.PersistDir) != "" {
		target.PersistDir = strings.TrimSpace(file.PersistDir)
	}
	if strings.TrimSpace(file.Collection) != "" {
		target.Collection = strings.TrimSpace(file.Collection)
	}
	if file.MinSimilarity != nil {
		target.MinSimilarity = *file.MinSimilarity
	}
	if strings.TrimSpace(file.EmbedderModel) != "" {
		target.EmbedderModel = strings.TrimSpace(file.EmbedderModel)
	}
	if strings.TrimSpace(file.EmbedderBaseURL) != "" {
		target.EmbedderBaseURL = strings.TrimSpace(file.EmbedderBaseURL)
	}
}

func mergeSchedulerConfig(target *SchedulerConfig, file *SchedulerFileConfig) {
	if target == nil || file == nil {
		return
	}
	if file.Enabled != nil {
		target.Enabled = *file.Enabled
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

func expandProactiveFileConfigEnv(lookup EnvLookup, file *ProactiveFileConfig) {
	if file == nil {
		return
	}
	if file.Memory != nil {
		if strings.TrimSpace(file.Memory.Store) != "" {
			file.Memory.Store = expandEnvValue(lookup, file.Memory.Store)
		}
		if file.Memory.Hybrid != nil {
			file.Memory.Hybrid.PersistDir = expandEnvValue(lookup, file.Memory.Hybrid.PersistDir)
			file.Memory.Hybrid.Collection = expandEnvValue(lookup, file.Memory.Hybrid.Collection)
			file.Memory.Hybrid.EmbedderModel = expandEnvValue(lookup, file.Memory.Hybrid.EmbedderModel)
			file.Memory.Hybrid.EmbedderBaseURL = expandEnvValue(lookup, file.Memory.Hybrid.EmbedderBaseURL)
		}
	}
	if file.Skills != nil {
		if file.Skills.Feedback != nil {
			file.Skills.Feedback.StorePath = expandEnvValue(lookup, file.Skills.Feedback.StorePath)
		}
	}
	if file.RAG != nil {
		file.RAG.PersistDir = expandEnvValue(lookup, file.RAG.PersistDir)
		file.RAG.Collection = expandEnvValue(lookup, file.RAG.Collection)
		file.RAG.EmbedderModel = expandEnvValue(lookup, file.RAG.EmbedderModel)
		file.RAG.EmbedderBaseURL = expandEnvValue(lookup, file.RAG.EmbedderBaseURL)
	}
	if file.Scheduler != nil {
		for i := range file.Scheduler.Triggers {
			file.Scheduler.Triggers[i].Task = expandEnvValue(lookup, file.Scheduler.Triggers[i].Task)
		}
	}
}
