// Package aiusage tracks per-user and per-tenant AI invocations across
// features. Records are written after each AI call completes; per-user counts
// are read pre-flight to enforce feature-specific daily caps. Per-tenant
// rollups (intended for billing visibility) are not yet wired up.
package aiusage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

type Store struct {
	db database.DB
}

func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

// Record persists a single AI invocation. Token counts may be zero if the
// provider did not return usage metadata — the call still counts toward the
// user's per-feature cap.
func (s *Store) Record(ctx context.Context, tenantID, userID uuid.UUID, feature string, promptTokens, responseTokens int32) error {
	_, err := s.db.Exec(ctx, "aiusage.Record",
		`INSERT INTO ai_usage (tenant_id, user_id, feature, prompt_tokens, response_tokens)
		 VALUES ($1, $2, $3, $4, $5)`,
		tenantID, userID, feature, promptTokens, responseTokens,
	)
	if err != nil {
		return fmt.Errorf("aiusage record: %w", err)
	}
	return nil
}

// CountUserSince returns the number of AI invocations a user has made against
// the given feature since `since`. Used for per-user daily cap enforcement;
// callers compute `since` (typically UTC midnight) to match their reset policy.
func (s *Store) CountUserSince(ctx context.Context, userID uuid.UUID, feature string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRow(ctx, "aiusage.CountUserSince",
		`SELECT COUNT(*) FROM ai_usage
		 WHERE user_id = $1 AND feature = $2 AND created_at >= $3`,
		userID, feature, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("aiusage count: %w", err)
	}
	return count, nil
}

// FeatureUsage is a per-feature rollup of invocation count and token totals
// across a time window. Returned by TenantUsageByFeature.
type FeatureUsage struct {
	Feature        string `json:"feature"`
	Calls          int64  `json:"calls"`
	PromptTokens   int64  `json:"prompt_tokens"`
	ResponseTokens int64  `json:"response_tokens"`
}

// TenantUsageByFeature returns AI usage for a tenant since `since`, grouped by
// feature. Intended for billing/admin visibility — sums are computed in the
// database to avoid pulling per-row data into the application.
func (s *Store) TenantUsageByFeature(ctx context.Context, tenantID uuid.UUID, since time.Time) ([]FeatureUsage, error) {
	rows, err := s.db.Query(ctx, "aiusage.TenantUsageByFeature",
		`SELECT feature,
		        COUNT(*) AS calls,
		        COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
		        COALESCE(SUM(response_tokens), 0) AS response_tokens
		 FROM ai_usage
		 WHERE tenant_id = $1 AND created_at >= $2
		 GROUP BY feature
		 ORDER BY feature`,
		tenantID, since,
	)
	if err != nil {
		return nil, fmt.Errorf("aiusage tenant rollup: %w", err)
	}
	defer rows.Close()

	var out []FeatureUsage
	for rows.Next() {
		var fu FeatureUsage
		if err := rows.Scan(&fu.Feature, &fu.Calls, &fu.PromptTokens, &fu.ResponseTokens); err != nil {
			return nil, fmt.Errorf("aiusage tenant rollup scan: %w", err)
		}
		out = append(out, fu)
	}
	return out, rows.Err()
}
