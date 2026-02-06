package http

import (
	"fmt"
	"reflect"
	"time"

	"alex/internal/workflow"
)

func sanitizeWorkflowNode(node workflow.NodeSnapshot) map[string]interface{} {
	sanitized := map[string]interface{}{
		"id":     node.ID,
		"status": node.Status,
	}

	if node.Error != "" {
		sanitized["error"] = node.Error
	}
	if !node.StartedAt.IsZero() {
		sanitized["started_at"] = node.StartedAt.Format(time.RFC3339Nano)
	}
	if !node.CompletedAt.IsZero() {
		sanitized["completed_at"] = node.CompletedAt.Format(time.RFC3339Nano)
	}
	if node.Duration > 0 {
		sanitized["duration"] = node.Duration
	}

	return sanitized
}

func sanitizeWorkflowSnapshot(snapshot *workflow.WorkflowSnapshot) map[string]interface{} {
	if snapshot == nil {
		return nil
	}

	sanitized := map[string]interface{}{
		"id":      snapshot.ID,
		"phase":   snapshot.Phase,
		"order":   snapshot.Order,
		"summary": snapshot.Summary,
	}

	if !snapshot.StartedAt.IsZero() {
		sanitized["started_at"] = snapshot.StartedAt.Format(time.RFC3339Nano)
	}
	if !snapshot.CompletedAt.IsZero() {
		sanitized["completed_at"] = snapshot.CompletedAt.Format(time.RFC3339Nano)
	}
	if snapshot.Duration > 0 {
		sanitized["duration"] = snapshot.Duration
	}

	return sanitized
}

func sanitizeValue(cache *DataCache, value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case string:
		return sanitizeStringValue(cache, typed)
	case []byte:
		return sanitizeStringValue(cache, string(typed))
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for key, val := range typed {
			sanitized[key] = sanitizeValue(cache, val)
		}
		return sanitized
	case []any:
		out := make([]any, len(typed))
		for i, entry := range typed {
			out[i] = sanitizeValue(cache, entry)
		}
		return out
	}

	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Map:
		return sanitizeMap(rv, cache)
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			bytesCopy := make([]byte, rv.Len())
			reflect.Copy(reflect.ValueOf(bytesCopy), rv)
			return sanitizeStringValue(cache, string(bytesCopy))
		}
		fallthrough
	case reflect.Array:
		sanitizedSlice := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			sanitizedSlice[i] = sanitizeValue(cache, rv.Index(i).Interface())
		}
		return sanitizedSlice
	case reflect.String:
		return sanitizeStringValue(cache, rv.String())
	default:
		return value
	}
}

func sanitizeMap(rv reflect.Value, cache *DataCache) map[string]interface{} {
	sanitized := make(map[string]interface{}, rv.Len())
	for _, key := range rv.MapKeys() {
		keyValue := key.Interface()
		keyString := fmt.Sprint(keyValue)
		sanitized[keyString] = sanitizeValue(cache, rv.MapIndex(key).Interface())
	}

	return sanitized
}

func sanitizeStringValue(cache *DataCache, value string) interface{} {
	if cache == nil {
		return value
	}

	if replaced := cache.MaybeStoreDataURI(value); replaced != nil {
		return replaced
	}

	return value
}
