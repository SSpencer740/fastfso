package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fastfso/fastfso/backend/internal/actionitem"
)

func strptr(s string) *string { return &s }

func TestBuildAdminPrompt_Empty(t *testing.T) {
	got := buildAdminPrompt(nil)
	if !strings.Contains(got, "no action items") {
		t.Errorf("expected empty-state message, got: %q", got)
	}
}

func TestBuildAdminPrompt_IncludesSubOrgAssigneeAndDue(t *testing.T) {
	due := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	items := []actionitem.ActionItemRow{
		{
			ID:           uuid.New(),
			Title:        "Renew FCL",
			Status:       "pending",
			Priority:     "high",
			SubOrgName:   strptr("Aerospace Division"),
			AssigneeName: strptr("Jane Doe"),
			DueDate:      &due,
			Description:  "Submit SF-328",
		},
	}

	got := buildAdminPrompt(items)

	for _, want := range []string{
		"Renew FCL",
		"sub-org: Aerospace Division",
		"assigned to: Jane Doe",
		"due: 2026-06-15",
		"details: Submit SF-328",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q\nfull prompt:\n%s", want, got)
		}
	}
}

func TestBuildAdminPrompt_DefaultsForNilFields(t *testing.T) {
	items := []actionitem.ActionItemRow{
		{ID: uuid.New(), Title: "Tenant-wide task", Status: "pending", Priority: "low"},
	}

	got := buildAdminPrompt(items)

	if !strings.Contains(got, "sub-org: tenant-wide") {
		t.Errorf("expected tenant-wide fallback, got:\n%s", got)
	}
	if !strings.Contains(got, "assigned to: unassigned") {
		t.Errorf("expected unassigned fallback, got:\n%s", got)
	}
	if strings.Contains(got, "due:") {
		t.Errorf("did not expect a due date for nil DueDate, got:\n%s", got)
	}
}
