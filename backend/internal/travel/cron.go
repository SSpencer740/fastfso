package travel

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/actionitem"
)

// addBusinessDays adds n business days (Mon–Fri) to t, skipping weekends.
func addBusinessDays(t time.Time, n int) time.Time {
	for n > 0 {
		t = t.AddDate(0, 0, 1)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			n--
		}
	}
	return t
}

// buildDebriefDescription formats the IC's answers into a readable FSO action item description.
func buildDebriefDescription(tripName string, p SubmitDebriefParams) string {
	yn := func(v bool) string {
		if v {
			return "Yes"
		}
		return "No"
	}
	detail := func(v bool, d string) string {
		if v && d != "" {
			return fmt.Sprintf(" — %s", d)
		}
		return ""
	}
	return fmt.Sprintf(
		"Post-travel debrief submitted for trip \"%s\".\n\n"+
			"1. Approached or contacted by a foreign national seeking sensitive information?\n   %s%s\n\n"+
			"2. Observed any suspicious surveillance or monitoring?\n   %s%s\n\n"+
			"3. Equipment, device, or sensitive material lost, stolen, or compromised?\n   %s%s\n\n"+
			"4. Received unusual or unexpected requests for information or access?\n   %s%s",
		tripName,
		yn(p.Q1ForeignContact), detail(p.Q1ForeignContact, p.Q1Details),
		yn(p.Q2Surveillance), detail(p.Q2Surveillance, p.Q2Details),
		yn(p.Q3EquipmentLoss), detail(p.Q3EquipmentLoss, p.Q3Details),
		yn(p.Q4UnusualRequests), detail(p.Q4UnusualRequests, p.Q4Details),
	)
}

// DebriefCron handles POST /api/cron/travel-debrief.
// It finds approved travel reports whose trip has fully ended and creates
// a pending debrief for the IC plus a tracking action item for the FSO.
func (h *Handler) DebriefCron(c *gin.Context) {
	ctx := c.Request.Context()

	reports, err := h.store.ListPendingDebriefReports(ctx)
	if err != nil {
		h.logger.Error("debrief cron: list pending reports", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	dueDate := addBusinessDays(time.Now(), 5)
	created := 0

	for _, r := range reports {
		debrief, err := h.store.CreateDebrief(ctx, r.ID, r.TenantID, r.UserID, &dueDate)
		if err != nil {
			h.logger.Error("debrief cron: create debrief", "report_id", r.ID, "error", err)
			continue
		}

		if err := h.store.MarkDebriefCreated(ctx, r.ID); err != nil {
			h.logger.Error("debrief cron: mark debrief created", "report_id", r.ID, "error", err)
			// Continue — debrief was created; we'll prevent re-processing via debrief_created_at on next run
		}

		// Notify the IC themselves. Without this, only the FSO knew the
		// debrief existed and the IC discovered it by happening to log in.
		if email, name, contactErr := h.store.GetUserContact(ctx, r.TenantID, r.UserID); contactErr == nil && email != "" {
			h.notify.NotifyDebriefAssigned(ctx, r.UserID, email, name, debrief.ID.String())
		}

		srcID := debrief.ID
		dd := dueDate
		cronAI, err := h.actionItems.Create(ctx, actionitem.CreateParams{
			TenantID:    r.TenantID,
			SourceType:  "travel_debrief",
			SourceID:    &srcID,
			Title:       fmt.Sprintf("Post-Travel Debrief Pending: %s", r.TripName),
			Description: fmt.Sprintf("IC has been sent a post-travel debrief questionnaire for trip \"%s\". Awaiting IC submission.", r.TripName),
			Priority:    "medium",
			DueDate:     &dd,
			SubOrgID:    r.SubOrgID,
		})
		if err != nil {
			h.logger.Error("debrief cron: create action item", "debrief_id", debrief.ID, "error", err)
		} else if cronAI != nil {
			for _, t := range h.actionItems.ResolveNotifyTargets(ctx, r.TenantID, cronAI) {
				tUserID, _ := uuid.Parse(t.UserID)
				h.notify.NotifyActionItemAssigned(ctx, tUserID, t.Email, t.Name, cronAI.SourceType)
			}
		}

		created++
	}

	c.JSON(http.StatusOK, gin.H{"created": created})
}
