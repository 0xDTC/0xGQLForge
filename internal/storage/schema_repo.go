package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/0xdtc/graphscope/internal/schema"
)

// SchemaRepo handles schema persistence.
type SchemaRepo struct {
	db *DB
}

// NewSchemaRepo creates a new schema repository.
func NewSchemaRepo(db *DB) *SchemaRepo {
	return &SchemaRepo{db: db}
}

// Save stores a schema with its raw and parsed JSON.
func (r *SchemaRepo) Save(s *schema.Schema, rawJSON string) error {
	parsed, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal schema: %w", err)
	}

	_, err = r.db.conn.Exec(
		`INSERT INTO schemas (id, name, source, raw_json, parsed_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name,
		   raw_json = excluded.raw_json,
		   parsed_json = excluded.parsed_json`,
		s.ID, s.Name, string(s.Source), rawJSON, string(parsed), s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert schema: %w", err)
	}
	return nil
}

// Get retrieves a schema by ID.
func (r *SchemaRepo) Get(id string) (*schema.Schema, error) {
	var parsedJSON string
	err := r.db.conn.QueryRow("SELECT parsed_json FROM schemas WHERE id = ?", id).Scan(&parsedJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query schema: %w", err)
	}

	var s schema.Schema
	if err := json.Unmarshal([]byte(parsedJSON), &s); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}
	return &s, nil
}

// GetRaw retrieves the original introspection JSON by schema ID.
func (r *SchemaRepo) GetRaw(id string) (string, error) {
	var raw string
	err := r.db.conn.QueryRow("SELECT raw_json FROM schemas WHERE id = ?", id).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query raw schema: %w", err)
	}
	return raw, nil
}

// List returns all schemas ordered by creation time (newest first).
func (r *SchemaRepo) List() ([]schema.Schema, error) {
	rows, err := r.db.conn.Query("SELECT parsed_json FROM schemas ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []schema.Schema
	for rows.Next() {
		var parsedJSON string
		if err := rows.Scan(&parsedJSON); err != nil {
			return nil, fmt.Errorf("scan schema: %w", err)
		}
		var s schema.Schema
		if err := json.Unmarshal([]byte(parsedJSON), &s); err != nil {
			return nil, fmt.Errorf("unmarshal schema: %w", err)
		}
		schemas = append(schemas, s)
	}
	return schemas, rows.Err()
}

// Delete removes a schema by ID.
func (r *SchemaRepo) Delete(id string) error {
	_, err := r.db.conn.Exec("DELETE FROM schemas WHERE id = ?", id)
	return err
}

// Count returns the total number of stored schemas.
func (r *SchemaRepo) Count() (int, error) {
	var count int
	err := r.db.conn.QueryRow("SELECT COUNT(*) FROM schemas").Scan(&count)
	return count, err
}

// TrafficRepo handles proxy traffic persistence.
type TrafficRepo struct {
	db *DB
}

// NewTrafficRepo creates a new traffic repository.
func NewTrafficRepo(db *DB) *TrafficRepo {
	return &TrafficRepo{db: db}
}

// Save stores a captured request.
func (r *TrafficRepo) Save(req *schema.CapturedRequest) error {
	headers, _ := json.Marshal(req.Headers)
	_, err := r.db.conn.Exec(
		`INSERT INTO traffic (id, timestamp, method, url, host, headers_json,
		  operation_name, query, variables_json, response_code, response_body,
		  fingerprint, cluster_id, schema_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.Timestamp, req.Method, req.URL, req.Host, string(headers),
		req.OperationName, req.Query, string(req.Variables),
		req.ResponseCode, req.ResponseBody,
		req.Fingerprint, req.ClusterID, req.SchemaID,
	)
	if err != nil {
		return fmt.Errorf("insert traffic: %w", err)
	}
	return nil
}

// List returns captured traffic, newest first. Limit 0 = no limit.
func (r *TrafficRepo) List(limit int) ([]schema.CapturedRequest, error) {
	q := "SELECT id, timestamp, method, url, host, headers_json, operation_name, query, variables_json, response_code, fingerprint, cluster_id FROM traffic ORDER BY timestamp DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := r.db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list traffic: %w", err)
	}
	defer rows.Close()

	var reqs []schema.CapturedRequest
	for rows.Next() {
		var req schema.CapturedRequest
		var headersJSON, varsJSON sql.NullString
		var clusterID, opName, query, fingerprint sql.NullString
		var respCode sql.NullInt64
		var ts time.Time

		if err := rows.Scan(
			&req.ID, &ts, &req.Method, &req.URL, &req.Host,
			&headersJSON, &opName, &query, &varsJSON, &respCode,
			&fingerprint, &clusterID,
		); err != nil {
			return nil, fmt.Errorf("scan traffic: %w", err)
		}

		req.Timestamp = ts
		if headersJSON.Valid {
			json.Unmarshal([]byte(headersJSON.String), &req.Headers)
		}
		if opName.Valid {
			req.OperationName = opName.String
		}
		if query.Valid {
			req.Query = query.String
		}
		if varsJSON.Valid {
			req.Variables = json.RawMessage(varsJSON.String)
		}
		if respCode.Valid {
			req.ResponseCode = int(respCode.Int64)
		}
		if fingerprint.Valid {
			req.Fingerprint = fingerprint.String
		}
		if clusterID.Valid {
			s := clusterID.String
			req.ClusterID = &s
		}

		reqs = append(reqs, req)
	}
	return reqs, rows.Err()
}

// ListByFingerprint returns all requests with the same structural fingerprint.
func (r *TrafficRepo) ListByFingerprint(fp string) ([]schema.CapturedRequest, error) {
	rows, err := r.db.conn.Query(
		"SELECT id, operation_name, query, variables_json FROM traffic WHERE fingerprint = ? ORDER BY timestamp DESC",
		fp,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []schema.CapturedRequest
	for rows.Next() {
		var req schema.CapturedRequest
		var opName, query, varsJSON sql.NullString
		if err := rows.Scan(&req.ID, &opName, &query, &varsJSON); err != nil {
			return nil, err
		}
		if opName.Valid {
			req.OperationName = opName.String
		}
		if query.Valid {
			req.Query = query.String
		}
		if varsJSON.Valid {
			req.Variables = json.RawMessage(varsJSON.String)
		}
		reqs = append(reqs, req)
	}
	return reqs, rows.Err()
}

// Count returns total captured traffic entries.
func (r *TrafficRepo) Count() (int, error) {
	var count int
	err := r.db.conn.QueryRow("SELECT COUNT(*) FROM traffic").Scan(&count)
	return count, err
}

// Clear deletes all captured traffic.
func (r *TrafficRepo) Clear() error {
	_, err := r.db.conn.Exec("DELETE FROM traffic")
	return err
}

// AnalysisRepo handles analysis results persistence.
type AnalysisRepo struct {
	db *DB
}

// NewAnalysisRepo creates a new analysis repository.
func NewAnalysisRepo(db *DB) *AnalysisRepo {
	return &AnalysisRepo{db: db}
}

// Save stores an analysis result.
func (r *AnalysisRepo) Save(id, schemaID, analysisType, resultJSON string) error {
	_, err := r.db.conn.Exec(
		`INSERT INTO analysis_results (id, schema_id, analysis_type, result_json)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET result_json = excluded.result_json`,
		id, schemaID, analysisType, resultJSON,
	)
	return err
}

// ListBySchema returns all analysis results for a schema.
func (r *AnalysisRepo) ListBySchema(schemaID string) (map[string]string, error) {
	rows, err := r.db.conn.Query(
		"SELECT analysis_type, result_json FROM analysis_results WHERE schema_id = ? ORDER BY created_at DESC",
		schemaID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]string)
	for rows.Next() {
		var aType, resultJSON string
		if err := rows.Scan(&aType, &resultJSON); err != nil {
			return nil, err
		}
		results[aType] = resultJSON
	}
	return results, rows.Err()
}
