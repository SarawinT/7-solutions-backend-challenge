// Package utils holds small, dependency-free helpers.
package utils

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Paginate converts a zero-based page and page size into an offset/limit
// pair, applying the default and maximum page size.
func Paginate(page, pageSize int) (offset, limit int) {
	limit = pageSize
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limit = min(limit, MaxPageSize)
	return max(page, 0) * limit, limit
}
