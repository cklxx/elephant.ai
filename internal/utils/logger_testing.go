package utils

import "sync"

// ResetLoggerForTests clears logger state for the provided category.
// It should only be used inside tests to ensure clean log files per run.
func ResetLoggerForTests(category LogCategory) {
	if category == LogCategoryService {
		if loggerInstance != nil {
			loggerInstance.Close()
		}
		loggerInstance = nil
		loggerOnce = sync.Once{}
		return
	}

	categoryMu.Lock()
	defer categoryMu.Unlock()
	if logger, ok := categoryLoggers[category]; ok {
		logger.Close()
		delete(categoryLoggers, category)
	}
}
