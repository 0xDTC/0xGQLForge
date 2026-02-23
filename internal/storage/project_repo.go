package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/0xDTC/0xGQLForge/internal/schema"
)

// ProjectRepo handles proxy project persistence.
type ProjectRepo struct {
	db *DB
}

// NewProjectRepo creates a new project repository.
func NewProjectRepo(db *DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

// Create stores a new project.
func (r *ProjectRepo) Create(p *schema.Project) error {
	_, err := r.db.conn.Exec(
		`INSERT INTO projects (id, name, proxy_addr, schema_id, traffic_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.ProxyAddr, p.SchemaID, p.TrafficCount, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// List returns all projects ordered by creation time (newest first).
// Traffic counts are computed live via a subquery.
func (r *ProjectRepo) List() ([]schema.Project, error) {
	rows, err := r.db.conn.Query(`
		SELECT p.id, p.name, p.proxy_addr, p.schema_id,
		       (SELECT COUNT(*) FROM traffic WHERE project_id = p.id) AS traffic_count,
		       p.created_at, p.updated_at
		FROM projects p
		ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []schema.Project
	for rows.Next() {
		var p schema.Project
		var proxyAddr, schemaID sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &proxyAddr, &schemaID, &p.TrafficCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		if proxyAddr.Valid {
			p.ProxyAddr = proxyAddr.String
		}
		if schemaID.Valid {
			s := schemaID.String
			p.SchemaID = &s
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Get retrieves a project by ID with live traffic count.
func (r *ProjectRepo) Get(id string) (*schema.Project, error) {
	var p schema.Project
	var proxyAddr, schemaID sql.NullString
	err := r.db.conn.QueryRow(`
		SELECT p.id, p.name, p.proxy_addr, p.schema_id,
		       (SELECT COUNT(*) FROM traffic WHERE project_id = p.id) AS traffic_count,
		       p.created_at, p.updated_at
		FROM projects p
		WHERE p.id = ?`, id,
	).Scan(&p.ID, &p.Name, &proxyAddr, &schemaID, &p.TrafficCount, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	if proxyAddr.Valid {
		p.ProxyAddr = proxyAddr.String
	}
	if schemaID.Valid {
		s := schemaID.String
		p.SchemaID = &s
	}
	return &p, nil
}

// Delete removes a project by ID.
func (r *ProjectRepo) Delete(id string) error {
	_, err := r.db.conn.Exec("DELETE FROM projects WHERE id = ?", id)
	return err
}

// UpdateSchema sets the inferred schema ID for a project.
func (r *ProjectRepo) UpdateSchema(projectID, schemaID string) error {
	_, err := r.db.conn.Exec(
		"UPDATE projects SET schema_id = ?, updated_at = ? WHERE id = ?",
		schemaID, time.Now().UTC(), projectID,
	)
	return err
}
