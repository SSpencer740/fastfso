package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/genai"

	"github.com/SSpencer740/fastfso/backend/internal/actionitem"
	"github.com/SSpencer740/fastfso/backend/internal/aiusage"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/env"
	"github.com/SSpencer740/fastfso/backend/internal/session"
	"github.com/SSpencer740/fastfso/backend/internal/task"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
)

// chatFeature is the aiusage feature key used to scope per-user limits and
// usage records for the chat endpoint.
const chatFeature = "chat"

// chatDailyMessageLimit caps how many chat invocations a single user can make
// in a UTC day. A hardcoded constant is intentional for v1; a per-tenant
// override comes later when there's a billing surface to attach it to.
const chatDailyMessageLimit = 100

const systemPrompt = `You are an AI assistant embedded in fastFSO, a security management platform for Facility Security Officers (FSOs) and cleared personnel operating under the National Industrial Security Program (NISP).

You have two primary functions:
1. Answer security-related questions from an FSO perspective, drawing on NISPOM, SEAD 3, SEAD 7, and general cleared industry best practices.
2. Summarize and answer questions about the user's action items provided below.

Key knowledge areas:
- Reportable foreign contacts: any contact with a foreign national where there is a continuing relationship or where the contact involves attempts to obtain classified or sensitive information, or where the contact is with someone from a country that poses a counterintelligence threat.
- Foreign travel: all foreign travel must be reported before departure. Post-travel debriefs are required.
- Reportable life events: marriage or cohabitation, financial difficulties or bankruptcy, foreign contacts, arrest or criminal charges, psychological counseling, drug or alcohol issues, and changes in foreign assets or accounts.
- Security violations: any loss, compromise, or suspected compromise of classified information must be reported immediately.
- Insider threat indicators and reporting obligations.
- Visit request procedures and VAL/VAR requirements.
- SF-86 and continuous evaluation obligations.

Always be helpful, accurate, and remind users to consult their FSO or security officer for official guidance on sensitive matters. Do not speculate about classified programs or specific clearance adjudications.

If asked about action items, refer only to the data provided — do not invent or assume additional items.`

type actionItemStore interface {
	List(ctx context.Context, f actionitem.ListFilters) ([]actionitem.ActionItemRow, int, error)
}

type taskStore interface {
	ListMyTasks(ctx context.Context, tenantID, userID uuid.UUID, search string, limit int) ([]task.MyTaskRow, error)
}

type Handler struct {
	items  actionItemStore
	tasks  taskStore
	store  *Store
	usage  *aiusage.Store
	client *genai.Client
	logger *slog.Logger
}

func NewHandler(items actionItemStore, tasks taskStore, store *Store, usage *aiusage.Store, client *genai.Client, logger *slog.Logger) *Handler {
	return &Handler{items: items, tasks: tasks, store: store, usage: usage, client: client, logger: logger}
}

// startOfUTCDay returns the most recent UTC midnight at or before now. The
// per-user cap resets at this boundary.
func startOfUTCDay(now time.Time) time.Time {
	t := now.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Usage handles GET /api/v1/me/chat/usage. Returns the caller's chat
// invocations today and the configured daily cap so the UI can show progress
// and warn before a 429.
func (h *Handler) Usage(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}
	used, err := h.usage.CountUserSince(c.Request.Context(), *sess.UserID, chatFeature, startOfUTCDay(time.Now()))
	if err != nil {
		h.logger.Error("chat usage count", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"used":  used,
		"limit": chatDailyMessageLimit,
	})
}

// NewClient creates a Vertex AI Gemini client using Application Default Credentials.
func NewClient(ctx context.Context) (*genai.Client, error) {
	return genai.NewClient(ctx, &genai.ClientConfig{
		Project:  env.GCPProjectID(),
		Location: env.VertexLocation(),
		Backend:  genai.BackendVertexAI,
	})
}

// Chat handles POST /api/v1/me/chat.
// Streams a Gemini response as newline-delimited JSON chunks.
func (h *Handler) Chat(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI assistant is not configured in this environment"})
		return
	}

	sess, ok := auth.GetSession(c)
	if !ok || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Message) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	ctx := c.Request.Context()
	tenantID := *sess.TenantID
	userID := *sess.UserID

	used, err := h.usage.CountUserSince(ctx, userID, chatFeature, startOfUTCDay(time.Now()))
	if err != nil {
		h.logger.Error("chat usage count", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if used >= chatDailyMessageLimit {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "daily AI chat limit reached, resets at 00:00 UTC",
			"limit": chatDailyMessageLimit,
		})
		return
	}

	fullSystem, err := h.fetchContext(ctx, sess)
	if err != nil {
		h.logger.Error("chat fetch context", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	history, err := h.store.GetHistory(ctx, tenantID, userID, 40)
	if err != nil {
		h.logger.Error("chat get history", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var contents []*genai.Content
	for _, msg := range history {
		role := genai.RoleUser
		if msg.Role == "assistant" {
			role = genai.RoleModel
		}
		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: msg.Content}},
		})
	}
	contents = append(contents, &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: body.Message}},
	})

	if err := h.store.SaveMessage(ctx, tenantID, userID, "user", body.Message); err != nil {
		h.logger.Error("chat save user message", "error", err)
	}

	model := h.client.Models
	stream := model.GenerateContentStream(
		ctx,
		env.GeminiModel(),
		contents,
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: fullSystem}},
			},
		},
	)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var assistantReply strings.Builder
	var promptTokens, responseTokens int32
	for resp, err := range stream {
		if err != nil {
			h.logger.Error("chat stream error", "error", err)
			chunk, _ := json.Marshal(map[string]string{"error": "stream error"})
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", chunk)
			c.Writer.Flush()
			return
		}
		for _, cand := range resp.Candidates {
			for _, part := range cand.Content.Parts {
				if part.Text == "" {
					continue
				}
				assistantReply.WriteString(part.Text)
				chunk, _ := json.Marshal(map[string]string{"text": part.Text})
				_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", chunk)
				c.Writer.Flush()
			}
		}
		// Vertex sends cumulative usage in the final chunk; intermediate
		// chunks may be nil or partial. Track the last non-nil value seen.
		if resp.UsageMetadata != nil {
			promptTokens = resp.UsageMetadata.PromptTokenCount
			responseTokens = resp.UsageMetadata.CandidatesTokenCount
		}
	}
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", `{"done":true}`)
	c.Writer.Flush()

	if reply := assistantReply.String(); reply != "" {
		if err := h.store.SaveMessage(context.Background(), tenantID, userID, "assistant", reply); err != nil {
			h.logger.Error("chat save assistant message", "error", err)
		}
	}

	// Record usage even when Vertex returned zero tokens — the call still
	// counts toward the user's daily cap. Use a detached context so a client
	// disconnect doesn't skip the write.
	if err := h.usage.Record(context.Background(), tenantID, userID, chatFeature, promptTokens, responseTokens); err != nil {
		h.logger.Error("chat record usage", "error", err)
	}
	telemetry.RecordAITokens(ctx, chatFeature, promptTokens, responseTokens)
}

func (h *Handler) fetchContext(ctx context.Context, sess *session.Session) (string, error) {
	role := ""
	if sess.UserRole != nil {
		role = *sess.UserRole
	}

	if role == "individual_contributor" {
		// Bound the AI context so a user with many tasks doesn't bloat the prompt.
		tasks, err := h.tasks.ListMyTasks(ctx, *sess.TenantID, *sess.UserID, "", 50)
		if err != nil {
			return "", err
		}
		return buildICPrompt(tasks), nil
	}

	f := actionitem.ListFilters{
		TenantID: *sess.TenantID,
		Limit:    50,
	}
	if role == "fso" || role == "read_only_fso" {
		f.SubOrgScope = sess.SubOrgID
	}
	items, _, err := h.items.List(ctx, f)
	if err != nil {
		return "", err
	}
	return buildAdminPrompt(items), nil
}

// History handles GET /api/v1/me/chat/history.
func (h *Handler) History(c *gin.Context) {
	sess, ok := auth.GetSession(c)
	if !ok || sess.TenantID == nil || sess.UserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no session"})
		return
	}

	msgs, err := h.store.GetHistory(c.Request.Context(), *sess.TenantID, *sess.UserID, 40)
	if err != nil {
		h.logger.Error("chat get history", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	type msgResponse struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]msgResponse, len(msgs))
	for i, m := range msgs {
		out[i] = msgResponse{Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}
	}
	c.JSON(http.StatusOK, gin.H{"messages": out})
}

// CleanupCron handles POST /api/cron/chat-cleanup.
func (h *Handler) CleanupCron(c *gin.Context) {
	n, err := h.store.CleanupOld(c.Request.Context())
	if err != nil {
		h.logger.Error("chat cleanup failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cleanup failed"})
		return
	}
	h.logger.Info("chat messages cleaned up", "deleted", n)
	c.JSON(http.StatusOK, gin.H{"deleted": n})
}

func buildICPrompt(tasks []task.MyTaskRow) string {
	if len(tasks) == 0 {
		return systemPrompt + "\n\nThe user currently has no tasks assigned to them."
	}
	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n## User's Assigned Tasks\n\n")
	for _, t := range tasks {
		sb.WriteString(fmt.Sprintf("- **%s** (status: %s, priority: %s", t.Title, t.Status, t.Priority))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf(", details: %s", t.Description))
		}
		sb.WriteString(")\n")
	}
	return sb.String()
}

func buildAdminPrompt(items []actionitem.ActionItemRow) string {
	if len(items) == 0 {
		return systemPrompt + "\n\nThere are currently no action items."
	}
	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n## Action Items\n\n")
	for _, it := range items {
		sb.WriteString(fmt.Sprintf("- **%s** (status: %s, priority: %s", it.Title, it.Status, it.Priority))
		if it.SubOrgName != nil {
			sb.WriteString(fmt.Sprintf(", sub-org: %s", *it.SubOrgName))
		} else {
			sb.WriteString(", sub-org: tenant-wide")
		}
		if it.AssigneeName != nil {
			sb.WriteString(fmt.Sprintf(", assigned to: %s", *it.AssigneeName))
		} else {
			sb.WriteString(", assigned to: unassigned")
		}
		if it.DueDate != nil {
			sb.WriteString(fmt.Sprintf(", due: %s", it.DueDate.Format("2006-01-02")))
		}
		if it.Description != "" {
			sb.WriteString(fmt.Sprintf(", details: %s", it.Description))
		}
		sb.WriteString(")\n")
	}
	return sb.String()
}
