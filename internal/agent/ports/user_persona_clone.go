package ports

import "maps"

// CloneUserPersonaProfile deep copies the provided user persona profile.
func CloneUserPersonaProfile(profile *UserPersonaProfile) *UserPersonaProfile {
	if profile == nil {
		return nil
	}
	cloned := *profile
	if len(profile.InitiativeSources) > 0 {
		cloned.InitiativeSources = append([]string(nil), profile.InitiativeSources...)
	}
	if len(profile.CoreDrives) > 0 {
		cloned.CoreDrives = append([]UserPersonaDrive(nil), profile.CoreDrives...)
	}
	if len(profile.TopDrives) > 0 {
		cloned.TopDrives = append([]string(nil), profile.TopDrives...)
	}
	if len(profile.Values) > 0 {
		cloned.Values = append([]string(nil), profile.Values...)
	}
	if len(profile.KeyChoices) > 0 {
		cloned.KeyChoices = append([]string(nil), profile.KeyChoices...)
	}
	if len(profile.ConstructionRules) > 0 {
		cloned.ConstructionRules = append([]string(nil), profile.ConstructionRules...)
	}
	if len(profile.Traits) > 0 {
		cloned.Traits = maps.Clone(profile.Traits)
	} else {
		cloned.Traits = nil
	}
	if len(profile.RawAnswers) > 0 {
		cloned.RawAnswers = clonePersonaAnswers(profile.RawAnswers)
	} else {
		cloned.RawAnswers = nil
	}
	return &cloned
}

func clonePersonaAnswers(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = clonePersonaValue(value)
	}
	return dst
}

func clonePersonaValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return clonePersonaAnswers(v)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = clonePersonaValue(item)
		}
		return out
	default:
		return v
	}
}
