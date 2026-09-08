package visitrequest

import (
	"encoding/csv"
	"time"
)

// writeExportHeader writes the CSV column header for the given detail profile.
// UseCRLF=true so Excel on Windows treats line endings correctly. Caller is
// responsible for the UTF-8 BOM if required.
//
// Two column profiles:
//   - "summary": 8 visit-only columns for quick at-a-glance
//   - "full" (default): 22 columns including snapshot-at-review fields and DD254 linkage
//
// Header and rows are written through the same *csv.Writer so the handler can
// stream rows one at a time (writeExportHeader once, then writeExportRow per row)
// without buffering the whole result set.
func writeExportHeader(cw *csv.Writer, detail string) {
	if detail == "summary" {
		_ = cw.Write([]string{
			"visit_id", "submitted_at", "submitter_name", "destination",
			"access_level", "status", "reviewed_at", "reviewer_name",
		})
		return
	}

	// "full" — audit-defensible layout. Column order groups visit → submitter
	// → snapshot-at-review → DD254 linkage → decision, matching how an
	// auditor reads each row left to right.
	_ = cw.Write([]string{
		"visit_id",
		"destination_name", "diss_smo_code", "visit_address",
		"visit_start_date", "visit_end_date",
		"access_level", "visit_description",
		"submitted_at", "status",
		"submitter_name", "submitter_email",
		"submitter_clearance_at_review", "submitter_investigation_type_at_review",
		"submitter_next_investigation_at_review",
		"dd254_contract_number", "dd254_prime_contractor",
		"dd254_classification_max", "dd254_period_start", "dd254_period_end",
		"authorization_confirmed_at_review",
		"reviewer_name", "reviewer_email", "reviewed_at", "reviewer_notes",
	})
}

// writeExportRow writes a single visit row in the given detail profile.
func writeExportRow(cw *csv.Writer, r ExportRow, detail string) {
	if detail == "summary" {
		_ = cw.Write([]string{
			r.VisitID.String(),
			r.SubmittedAt.UTC().Format(time.RFC3339),
			r.SubmitterName,
			r.DestinationName,
			r.AccessLevel,
			r.Status,
			fmtTime(r.ReviewedAt),
			strDeref(r.ReviewerName),
		})
		return
	}

	_ = cw.Write([]string{
		r.VisitID.String(),
		r.DestinationName, r.DissSmoCode, r.VisitAddress,
		r.VisitStartDate.UTC().Format("2006-01-02"),
		r.VisitEndDate.UTC().Format("2006-01-02"),
		r.AccessLevel, r.VisitDescription,
		r.SubmittedAt.UTC().Format(time.RFC3339), r.Status,
		r.SubmitterName, r.SubmitterEmail,
		strDeref(r.ClearanceAtReview), strDeref(r.InvestigationTypeAtReview),
		fmtTime(r.NextInvestigationAtReview),
		strDeref(r.DD254ContractNumber), strDeref(r.DD254PrimeContractor),
		strDeref(r.DD254Classification),
		fmtTime(r.DD254PeriodStart), fmtTime(r.DD254PeriodEnd),
		boolStr(r.AuthorizationConfirmedAtReview),
		strDeref(r.ReviewerName), strDeref(r.ReviewerEmail),
		fmtTime(r.ReviewedAt), r.ReviewerNotes,
	})
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
