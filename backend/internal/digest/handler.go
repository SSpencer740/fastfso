package digest

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CronHandler runs one digest pass. Mounted under /api/cron and protected by
// the Cloud Scheduler middleware.
func (s *Service) CronHandler(c *gin.Context) {
	if err := s.Run(c.Request.Context()); err != nil {
		s.logger.Error("digest run failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "digest run failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
