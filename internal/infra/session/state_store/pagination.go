package state_store

import "strconv"

const defaultPageSize = 20

// paginateAsc windows a sorted-ascending slice of turn IDs using a cursor
// that marks the last returned turn. Returns the selected window and the
// next cursor (empty when no more pages).
func paginateAsc(turnIDs []int, cursor string, limit int) ([]int, string) {
	if len(turnIDs) == 0 {
		return nil, ""
	}
	startIdx := 0
	if cursor != "" {
		if cursorID, err := strconv.Atoi(cursor); err == nil {
			startIdx = len(turnIDs)
			for i, id := range turnIDs {
				if id > cursorID {
					startIdx = i
					break
				}
			}
		}
	}
	if startIdx >= len(turnIDs) {
		return nil, ""
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	end := startIdx + limit
	if end > len(turnIDs) {
		end = len(turnIDs)
	}
	var nextCursor string
	if end < len(turnIDs) {
		nextCursor = strconv.Itoa(turnIDs[end-1])
	}
	return turnIDs[startIdx:end], nextCursor
}

// paginateDesc windows a sorted-descending slice of turn IDs. The cursor
// marks the last returned turn and subsequent items have smaller IDs.
func paginateDesc(turnIDs []int, cursor string, limit int) ([]int, string) {
	if len(turnIDs) == 0 {
		return nil, ""
	}
	startIdx := 0
	if cursor != "" {
		if cursorID, err := strconv.Atoi(cursor); err == nil {
			startIdx = len(turnIDs)
			for i, id := range turnIDs {
				if id < cursorID {
					startIdx = i
					break
				}
			}
		}
	}
	if startIdx >= len(turnIDs) {
		return nil, ""
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	end := startIdx + limit
	if end > len(turnIDs) {
		end = len(turnIDs)
	}
	var nextCursor string
	if end < len(turnIDs) {
		nextCursor = strconv.Itoa(turnIDs[end-1])
	}
	return turnIDs[startIdx:end], nextCursor
}
