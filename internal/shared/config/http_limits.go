package config

func applyHTTPLimitsFileConfig(cfg *RuntimeConfig, meta *Metadata, file *HTTPLimitsFileConfig) {
	if cfg == nil || file == nil {
		return
	}
	limits := &cfg.HTTPLimits
	if file.DefaultMaxResponseBytes != nil {
		limits.DefaultMaxResponseBytes = *file.DefaultMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.default_max_response_bytes"] = SourceFile
		}
	}
	if file.WebFetchMaxResponseBytes != nil {
		limits.WebFetchMaxResponseBytes = *file.WebFetchMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.web_fetch_max_response_bytes"] = SourceFile
		}
	}
	if file.WebSearchMaxResponseBytes != nil {
		limits.WebSearchMaxResponseBytes = *file.WebSearchMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.web_search_max_response_bytes"] = SourceFile
		}
	}
	if file.MusicSearchMaxResponseBytes != nil {
		limits.MusicSearchMaxResponseBytes = *file.MusicSearchMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.music_search_max_response_bytes"] = SourceFile
		}
	}
	if file.ModelListMaxResponseBytes != nil {
		limits.ModelListMaxResponseBytes = *file.ModelListMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.model_list_max_response_bytes"] = SourceFile
		}
	}
	if file.SandboxMaxResponseBytes != nil {
		limits.SandboxMaxResponseBytes = *file.SandboxMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.sandbox_max_response_bytes"] = SourceFile
		}
	}
}

func applyHTTPLimitsOverrides(cfg *RuntimeConfig, meta *Metadata, overrides *HTTPLimitsOverrides) {
	if cfg == nil || overrides == nil {
		return
	}
	limits := &cfg.HTTPLimits
	if overrides.DefaultMaxResponseBytes != nil {
		limits.DefaultMaxResponseBytes = *overrides.DefaultMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.default_max_response_bytes"] = SourceOverride
		}
	}
	if overrides.WebFetchMaxResponseBytes != nil {
		limits.WebFetchMaxResponseBytes = *overrides.WebFetchMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.web_fetch_max_response_bytes"] = SourceOverride
		}
	}
	if overrides.WebSearchMaxResponseBytes != nil {
		limits.WebSearchMaxResponseBytes = *overrides.WebSearchMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.web_search_max_response_bytes"] = SourceOverride
		}
	}
	if overrides.MusicSearchMaxResponseBytes != nil {
		limits.MusicSearchMaxResponseBytes = *overrides.MusicSearchMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.music_search_max_response_bytes"] = SourceOverride
		}
	}
	if overrides.ModelListMaxResponseBytes != nil {
		limits.ModelListMaxResponseBytes = *overrides.ModelListMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.model_list_max_response_bytes"] = SourceOverride
		}
	}
	if overrides.SandboxMaxResponseBytes != nil {
		limits.SandboxMaxResponseBytes = *overrides.SandboxMaxResponseBytes
		if meta != nil {
			meta.sources["http_limits.sandbox_max_response_bytes"] = SourceOverride
		}
	}
}
