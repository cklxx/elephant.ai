package config

func applyHTTPLimitsFileConfig(cfg *RuntimeConfig, meta *Metadata, file *HTTPLimitsFileConfig) {
	if cfg == nil || file == nil {
		return
	}
	applyHTTPLimitsValues(&cfg.HTTPLimits, meta, SourceFile, httpLimitValues{
		defaultMaxResponseBytes:     file.DefaultMaxResponseBytes,
		webFetchMaxResponseBytes:    file.WebFetchMaxResponseBytes,
		webSearchMaxResponseBytes:   file.WebSearchMaxResponseBytes,
		musicSearchMaxResponseBytes: file.MusicSearchMaxResponseBytes,
		modelListMaxResponseBytes:   file.ModelListMaxResponseBytes,
	})
}

func applyHTTPLimitsOverrides(cfg *RuntimeConfig, meta *Metadata, overrides *HTTPLimitsOverrides) {
	if cfg == nil || overrides == nil {
		return
	}
	applyHTTPLimitsValues(&cfg.HTTPLimits, meta, SourceOverride, httpLimitValues{
		defaultMaxResponseBytes:     overrides.DefaultMaxResponseBytes,
		webFetchMaxResponseBytes:    overrides.WebFetchMaxResponseBytes,
		webSearchMaxResponseBytes:   overrides.WebSearchMaxResponseBytes,
		musicSearchMaxResponseBytes: overrides.MusicSearchMaxResponseBytes,
		modelListMaxResponseBytes:   overrides.ModelListMaxResponseBytes,
	})
}

type httpLimitValues struct {
	defaultMaxResponseBytes     *int
	webFetchMaxResponseBytes    *int
	webSearchMaxResponseBytes   *int
	musicSearchMaxResponseBytes *int
	modelListMaxResponseBytes   *int
}

func applyHTTPLimitsValues(limits *HTTPLimitsConfig, meta *Metadata, source ValueSource, values httpLimitValues) {
	if limits == nil {
		return
	}
	applyHTTPLimitValue(&limits.DefaultMaxResponseBytes, values.defaultMaxResponseBytes, meta, "http_limits.default_max_response_bytes", source)
	applyHTTPLimitValue(&limits.WebFetchMaxResponseBytes, values.webFetchMaxResponseBytes, meta, "http_limits.web_fetch_max_response_bytes", source)
	applyHTTPLimitValue(&limits.WebSearchMaxResponseBytes, values.webSearchMaxResponseBytes, meta, "http_limits.web_search_max_response_bytes", source)
	applyHTTPLimitValue(&limits.MusicSearchMaxResponseBytes, values.musicSearchMaxResponseBytes, meta, "http_limits.music_search_max_response_bytes", source)
	applyHTTPLimitValue(&limits.ModelListMaxResponseBytes, values.modelListMaxResponseBytes, meta, "http_limits.model_list_max_response_bytes", source)
}

func applyHTTPLimitValue(dst *int, src *int, meta *Metadata, key string, source ValueSource) {
	if src == nil {
		return
	}
	*dst = *src
	if meta != nil {
		meta.sources[key] = source
	}
}
