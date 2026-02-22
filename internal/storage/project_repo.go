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
func (r *ProjectRepo) List() ([]schema.Project, error) {
	rows, err := r.db.conn.Query(
		`SELECT id, name, proxy_addr, schema_id, traffic_count, created_at, updated_at
		 FROM projects ORDER BY created_at DESC`,
	)
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

// Get retrieves a project by ID.
func (r *ProjectRepo) Get(id string) (*schema.Project, error) {
	var p schema.Project
	var proxyAddr, schemaID sql.NullString
	err := r.db.conn.QueryRow(
		`SELECT id, name, proxy_addr, schema_id, traffic_count, created_at, updated_at
		 FROM projects WHERE id = ?`, id,
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

// UpdateSchema sets the inferred schema ID for a project.
func (r *ProjectRepo) UpdateSchema(projectID, schemaID string) error {
	_, err := r.db.conn.Exec(
		"UPDATE projects SET schema_id = ?, updated_at = ? WHERE id = ?",
		schemaID, time.Now().UTC(), projectID,
	)
	return err
}

// IncrementTrafficCount increments traffic_count and bumps updated_at.
func (r *ProjectRepo) IncrementTrafficCount(id string) error {
	_, err := r.db.conn.Exec(
		"UPDATE projects SET traffic_count = traffic_count + 1, updated_at = ? WHERE id = ?",
		time.Now().UTC(), id,
	)
	return err
}
