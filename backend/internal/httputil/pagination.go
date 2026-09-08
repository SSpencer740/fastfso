package httputil

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const maxPageLimit = 200

// ParsePagination reads limit and offset from query params with safe defaults.
// limit is clamped to [1, maxPageLimit]; offset is clamped to >= 0.
func ParsePagination(c *gin.Context, defaultLimit int) (limit, offset int) {
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 {
		limit = 1
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
