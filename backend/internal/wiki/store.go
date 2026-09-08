package wiki

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/SSpencer740/fastfso/backend/internal/database"
)

var ErrNotFound = errors.New("wiki post not found")

// Store wraps database.DB for wiki operations.
type Store struct {
	db database.DB
}

// NewStore creates a new wiki Store.
func NewStore(db database.DB) *Store {
	return &Store{db: db}
}

const postColumns = `
	wp.id, wp.tenant_id, wp.created_by_user_id, wp.sub_org_id, wp.title, wp.content,
	wp.published, i.name, wp.created_at, wp.updated_at`

const postJoin = `
	FROM wiki_posts wp
	JOIN users u ON u.id = wp.created_by_user_id
	JOIN identities i ON i.id = u.identity_id`

func scanPost(row interface{ Scan(...any) error }) (*Post, error) {
	var p Post
	if err := row.Scan(
		&p.ID, &p.TenantID, &p.CreatedByUserID, &p.SubOrgID, &p.Title, &p.Content,
		&p.Published, &p.CreatorName, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	p.Files = []PostFile{}
	return &p, nil
}

// listFiles loads all files for a set of post IDs and populates the Files slice.
func (s *Store) listFiles(ctx context.Context, posts []*Post) error {
	if len(posts) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(posts))
	index := make(map[uuid.UUID]*Post, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
		index[p.ID] = p
	}

	rows, err := s.db.Query(ctx, "wiki.listFiles",
		`SELECT id, post_id, file_name, file_size, created_at
		 FROM wiki_post_files WHERE post_id = ANY($1) ORDER BY created_at`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("wiki list files: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var f PostFile
		if err := rows.Scan(&f.ID, &f.PostID, &f.FileName, &f.FileSize, &f.CreatedAt); err != nil {
			return fmt.Errorf("wiki list files scan: %w", err)
		}
		p := index[f.PostID]
		p.Files = append(p.Files, f)
	}
	return nil
}

// Create inserts a new wiki post.
func (s *Store) Create(ctx context.Context, p CreateParams) (*Post, error) {
	row := s.db.QueryRow(ctx, "wiki.Create",
		`INSERT INTO wiki_posts (tenant_id, created_by_user_id, title, content, published, sub_org_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		p.TenantID, p.CreatedByUserID, p.Title, p.Content, p.Published, p.SubOrgID,
	)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return nil, fmt.Errorf("wiki create: %w", err)
	}
	return s.Get(ctx, p.TenantID, id)
}

// Get retrieves a single wiki post by ID scoped to the tenant, including files.
func (s *Store) Get(ctx context.Context, tenantID, id uuid.UUID) (*Post, error) {
	row := s.db.QueryRow(ctx, "wiki.Get",
		`SELECT`+postColumns+postJoin+`
		 WHERE wp.tenant_id = $1 AND wp.id = $2`,
		tenantID, id,
	)
	p, err := scanPost(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("wiki get: %w", err)
	}
	if err := s.listFiles(ctx, []*Post{p}); err != nil {
		return nil, err
	}
	return p, nil
}

// List returns all posts for a tenant. Admins see all; ICs see only published.
// subOrgScope, when non-nil, filters to posts in that sub-org or tenant-wide (sub_org_id IS NULL).
func (s *Store) List(ctx context.Context, tenantID uuid.UUID, publishedOnly bool, subOrgScope *uuid.UUID) ([]*Post, error) {
	where := "wp.tenant_id = $1"
	args := []any{tenantID}
	if publishedOnly {
		where += " AND wp.published = true"
	}
	if subOrgScope != nil {
		where += " AND (wp.sub_org_id IS NULL OR wp.sub_org_id = $2)"
		args = append(args, subOrgScope)
	}
	rows, err := s.db.Query(ctx, "wiki.List",
		`SELECT`+postColumns+postJoin+`
		 WHERE `+where+`
		 ORDER BY wp.created_at DESC`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("wiki list: %w", err)
	}
	defer rows.Close()

	var result []*Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("wiki list scan: %w", err)
		}
		result = append(result, p)
	}
	if err := s.listFiles(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

// Update modifies title, content, published state, and optionally sub_org_id
// on an existing post. sub_org_id is only touched when p.UpdateSubOrgID is
// true, so callers without permission to move posts between sub-orgs can
// leave the column untouched without re-fetching its current value.
func (s *Store) Update(ctx context.Context, p UpdateParams) (*Post, error) {
	var (
		tag pgconn.CommandTag
		err error
	)
	if p.UpdateSubOrgID {
		tag, err = s.db.Exec(ctx, "wiki.Update",
			`UPDATE wiki_posts
			 SET title = $1, content = $2, published = $3, sub_org_id = $4, updated_at = now()
			 WHERE id = $5 AND tenant_id = $6`,
			p.Title, p.Content, p.Published, p.SubOrgID, p.ID, p.TenantID,
		)
	} else {
		tag, err = s.db.Exec(ctx, "wiki.Update",
			`UPDATE wiki_posts
			 SET title = $1, content = $2, published = $3, updated_at = now()
			 WHERE id = $4 AND tenant_id = $5`,
			p.Title, p.Content, p.Published, p.ID, p.TenantID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("wiki update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return s.Get(ctx, p.TenantID, p.ID)
}

// Delete removes a wiki post permanently (files cascade via FK).
func (s *Store) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "wiki.Delete",
		`DELETE FROM wiki_posts WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	if err != nil {
		return fmt.Errorf("wiki delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddFile inserts a file record for a wiki post and returns it.
func (s *Store) AddFile(ctx context.Context, postID, tenantID uuid.UUID, fileName, storageKey string, fileSize int64) (*PostFile, error) {
	var f PostFile
	err := s.db.QueryRow(ctx, "wiki.AddFile",
		`INSERT INTO wiki_post_files (post_id, tenant_id, file_name, storage_key, file_size)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, post_id, file_name, file_size, created_at`,
		postID, tenantID, fileName, storageKey, fileSize,
	).Scan(&f.ID, &f.PostID, &f.FileName, &f.FileSize, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("wiki add file: %w", err)
	}
	return &f, nil
}

// GetFile retrieves a file record and its storage key, scoped to the tenant.
func (s *Store) GetFile(ctx context.Context, tenantID, fileID uuid.UUID) (storageKey string, f PostFile, err error) {
	err = s.db.QueryRow(ctx, "wiki.GetFile",
		`SELECT f.id, f.post_id, f.file_name, f.file_size, f.created_at, f.storage_key
		 FROM wiki_post_files f
		 WHERE f.id = $1 AND f.tenant_id = $2`,
		fileID, tenantID,
	).Scan(&f.ID, &f.PostID, &f.FileName, &f.FileSize, &f.CreatedAt, &storageKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", PostFile{}, ErrNotFound
		}
		return "", PostFile{}, fmt.Errorf("wiki get file: %w", err)
	}
	return storageKey, f, nil
}

// DeleteFile removes a file record and returns its storage key so the caller can clean up storage.
func (s *Store) DeleteFile(ctx context.Context, tenantID, fileID uuid.UUID) (storageKey string, err error) {
	err = s.db.QueryRow(ctx, "wiki.DeleteFile",
		`DELETE FROM wiki_post_files WHERE id = $1 AND tenant_id = $2 RETURNING storage_key`,
		fileID, tenantID,
	).Scan(&storageKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("wiki delete file: %w", err)
	}
	return storageKey, nil
}

// GetFSO returns the name and email of the FSO the viewer should see as
// their point of contact. The lookup is sub-org-aware: if subOrgID is
// non-nil and that sub-org has an assigned primary FSO, we return them.
// Otherwise we fall back to the oldest FSO-role user in the tenant — the
// pre-primary-FSO behavior. Returns nil if no FSO can be found at all.
func (s *Store) GetFSO(ctx context.Context, tenantID uuid.UUID, subOrgID *uuid.UUID) (*FSOInfo, error) {
	if subOrgID != nil {
		var f FSOInfo
		err := s.db.QueryRow(ctx, "wiki.GetFSO.primary",
			`SELECT i.name, i.email
			 FROM suborganizations s
			 JOIN users u ON u.id = s.primary_fso_user_id
			 JOIN identities i ON i.id = u.identity_id
			 WHERE s.id = $1 AND s.tenant_id = $2`,
			*subOrgID, tenantID,
		).Scan(&f.Name, &f.Email)
		if err == nil {
			return &f, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("wiki get primary fso: %w", err)
		}
		// No primary FSO assigned to this sub-org — fall through to
		// tenant-wide fallback below.
	}

	var f FSOInfo
	err := s.db.QueryRow(ctx, "wiki.GetFSO.fallback",
		`SELECT i.name, i.email
		 FROM users u
		 JOIN identities i ON i.id = u.identity_id
		 WHERE u.tenant_id = $1 AND u.role = 'fso'
		 ORDER BY u.created_at
		 LIMIT 1`,
		tenantID,
	).Scan(&f.Name, &f.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("wiki get fso: %w", err)
	}
	return &f, nil
}
