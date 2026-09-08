package task

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
)

const exportDateLayout = "2006-01-02"

// ExportTasks handles GET /api/tasks/export?format=csv|pdf&from=YYYY-MM-DD&to=YYYY-MM-DD
func (h *Handler) ExportTasks(c *gin.Context) {
	_, tenantID, _, ok := sessionInfo(c)
	if !ok {
		return
	}

	format := c.Query("format")
	if format != "csv" && format != "pdf" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be csv or pdf"})
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to date parameters are required"})
		return
	}

	from, err := time.Parse(exportDateLayout, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date, use YYYY-MM-DD"})
		return
	}
	to, err := time.Parse(exportDateLayout, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date, use YYYY-MM-DD"})
		return
	}
	// Make `to` inclusive by extending to end of day.
	to = to.Add(24*time.Hour - time.Second)

	if to.Before(from) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to must be on or after from"})
		return
	}

	rows, err := h.store.ExportByDateRange(c.Request.Context(), tenantID, from, to)
	if err != nil {
		h.logger.Error("export tasks failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	filename := fmt.Sprintf("tasks_%s_%s", fromStr, toStr)

	switch format {
	case "csv":
		h.writeCSV(c, filename+".csv", rows)
	case "pdf":
		h.writePDF(c, filename+".pdf", fromStr, toStr, rows)
	}
}

func (h *Handler) writeCSV(c *gin.Context, filename string, rows []ExportRow) {
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Status(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"Task", "Sub-Organization", "Assignee", "Email", "Status", "Due Date", "Completed Date"})

	for _, r := range rows {
		_ = w.Write([]string{
			r.TaskTitle,
			r.SubOrgName,
			r.AssigneeName,
			r.AssigneeEmail,
			r.Status,
			formatDate(r.DueDate),
			formatDate(r.CompletedAt),
		})
	}
	w.Flush()
}

func (h *Handler) writePDF(c *gin.Context, filename, fromStr, toStr string, rows []ExportRow) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 8, "Training Task Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(0, 6, fmt.Sprintf("Date range: %s to %s", fromStr, toStr), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Column widths for landscape A4 (277mm usable)
	cols := []string{"Task", "Sub-Org", "Assignee", "Email", "Status", "Due Date", "Completed"}
	widths := []float64{70, 35, 35, 55, 22, 25, 25}

	// Header row
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(59, 130, 246) // blue-500
	pdf.SetTextColor(255, 255, 255)
	for i, col := range cols {
		pdf.CellFormat(widths[i], 7, col, "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)

	// Data rows
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(0, 0, 0)
	fill := false
	for _, r := range rows {
		if fill {
			pdf.SetFillColor(243, 244, 246) // gray-100
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		cells := []string{
			r.TaskTitle,
			r.SubOrgName,
			r.AssigneeName,
			r.AssigneeEmail,
			r.Status,
			formatDate(r.DueDate),
			formatDate(r.CompletedAt),
		}
		for i, cell := range cells {
			pdf.CellFormat(widths[i], 6, truncate(cell, 40), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}

	if len(rows) == 0 {
		pdf.SetFont("Helvetica", "I", 8)
		pdf.CellFormat(0, 6, "No tasks found for the selected date range.", "", 1, "L", false, 0, "")
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/pdf")
	c.Status(http.StatusOK)
	_ = pdf.Output(c.Writer)
}

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
