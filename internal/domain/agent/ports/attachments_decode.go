package ports

import "encoding/json"

// CoerceAttachmentMap converts a raw payload into a typed attachment map.
// Live events may already carry map[string]Attachment, while replayed events
// often decode into map[string]any with Attachment-shaped entries.
func CoerceAttachmentMap(raw any) map[string]Attachment {
	if raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]Attachment:
		if len(typed) == 0 {
			return nil
		}
		return typed
	case map[string]any:
		if len(typed) == 0 {
			return nil
		}
		result := make(map[string]Attachment, len(typed))
		for key, value := range typed {
			att, ok := AttachmentFromAny(value)
			if !ok {
				continue
			}
			if att.Name == "" {
				att.Name = key
			}
			result[key] = att
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

// AttachmentFromAny converts a generic map payload into an Attachment.
func AttachmentFromAny(raw any) (Attachment, bool) {
	switch typed := raw.(type) {
	case Attachment:
		return typed, true
	case map[string]any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return Attachment{}, false
		}
		var att Attachment
		if err := json.Unmarshal(encoded, &att); err != nil {
			return Attachment{}, false
		}
		return att, true
	default:
		return Attachment{}, false
	}
}
