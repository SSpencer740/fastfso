package aiusage

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	store  *Store
	logger *slog.Logger
}

func NewHandler(store *Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

// startOfUTCMonth returns the first instant of the current UTC month. Admin
// rollups default to this window for billing-period alignment.
func startOfUTCMonth(now time.Time) time.Time {
	t := now.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// AdminTenantUsage handles GET /api/admin/tenants/:id/ai-usage. Returns the
// current-UTC-month rollup of AI calls and token totals for the tenant,
// grouped by feature. Super-admin only — route group enforces this.
func (h *Handler) AdminTenantUsage(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}

	since := startOfUTCMonth(time.Now())
	features, err := h.store.TenantUsageByFeature(c.Request.Context(), tenantID, since)
	if err != nil {
		h.logger.Error("admin tenant ai usage", "tenant_id", tenantID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if features == nil {
		features = []FeatureUsage{}
	}
	c.JSON(http.StatusOK, gin.H{
		"since":    since,
		"features": features,
	})
}
