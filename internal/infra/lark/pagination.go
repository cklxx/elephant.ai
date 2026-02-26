package lark

func normalizePageSize(pageSize int, fallback int) int {
	if pageSize > 0 {
		return pageSize
	}
	return fallback
}

func extractPageTokenAndHasMore(pageToken *string, hasMore *bool) (string, bool) {
	var token string
	var more bool
	if pageToken != nil {
		token = *pageToken
	}
	if hasMore != nil {
		more = *hasMore
	}
	return token, more
}
