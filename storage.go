package knowledgegraph

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	sqliteFilename       = "knowledgegraph.sqlite"
	legacySQLiteFilename = "octosucker.sqlite"
	tableNodes           = "kg_nodes"
	tableEdges           = "kg_edges"
)

type NodeRow struct {
	ID         string    `json:"id"`
	NodeType   string    `json:"node_type"`
	AliasesJSON string   `json:"aliases_json"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updated_at"`
	Embedding  []byte    `json:"-"`
}

type EdgeRow struct {
	ID                int64      `json:"id"`
	FromID            string     `json:"from_id"`
	ToID              string     `json:"to_id"`
	GraphKind         string     `json:"graph_kind"`
	RelationType      string     `json:"relation_type"`
	Polarity          int        `json:"polarity"`
	Confidence        float64    `json:"confidence"`
	ConditionText     string     `json:"condition_text"`
	SourceType        string     `json:"source_type"`
	SourceRef         string     `json:"source_ref"`
	EvidenceCount     int        `json:"evidence_count"`
	FailedCount       int        `json:"failed_count"`
	CreatedAt         time.Time  `json:"created_at"`
	ObservedAt        *time.Time `json:"observed_at,omitempty"`
	ValidFrom         *time.Time `json:"valid_from,omitempty"`
	ValidUntil        *time.Time `json:"valid_until,omitempty"`
	LastVerifiedAt    *time.Time `json:"last_verified_at,omitempty"`
	DecayHalfLifeDays int        `json:"decay_half_life_days"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	IsExecutable      bool       `json:"is_executable"`
	ActivationRule    string     `json:"activation_rule"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type EdgeInput struct {
	FromID            string
	ToID              string
	GraphKind         string
	RelationType      string
	Polarity          int
	Confidence        float64
	ConditionText     string
	SourceType        string
	SourceRef         string
	ObservedAt        *time.Time
	ValidFrom         *time.Time
	ValidUntil        *time.Time
	DecayHalfLifeDays int
	ExpiresAt         *time.Time
	IsExecutable      bool
	ActivationRule    string
}

type EdgeEvidenceInput struct {
	EdgeID      int64
	SourceType  string
	SourceRef   string
	Snippet     string
	ObservedAt  *time.Time
	Supports    bool
	Weight      float64
}

type Store struct {
	conn *sql.DB
}

type StoreConfig struct {
	WorkspaceRoot string
	DBPath        string
}

func OpenStore(cfg StoreConfig) (*Store, error) {
	dbPath, err := resolveDBPath(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("knowledgegraph: mkdir db dir: %w", err)
	}
	dsn := sqliteDSN(dbPath)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("knowledgegraph: open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("knowledgegraph: ping sqlite: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("knowledgegraph: pragma journal_mode: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("knowledgegraph: pragma busy_timeout: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("knowledgegraph: pragma foreign_keys: %w", err)
	}
	s := &Store{conn: sqlDB}
	if err := s.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("knowledgegraph: migrate: %w", err)
	}
	return s, nil
}

func sqliteDSN(dbPath string) string {
	clean := filepath.ToSlash(filepath.Clean(dbPath))
	if strings.HasPrefix(clean, "file:") {
		return clean
	}
	return "file:" + clean
}

func resolveDBPath(cfg StoreConfig) (string, error) {
	if cfg.DBPath != "" {
		return filepath.Clean(cfg.DBPath), nil
	}
	if cfg.WorkspaceRoot == "" {
		return "", fmt.Errorf("knowledgegraph: db_path or workspace is required")
	}
	dataDir := filepath.Join(cfg.WorkspaceRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("knowledgegraph: mkdir data dir: %w", err)
	}
	current := filepath.Join(dataDir, sqliteFilename)
	if _, err := os.Stat(current); err == nil {
		return current, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("knowledgegraph: stat sqlite: %w", err)
	}
	legacy := filepath.Join(dataDir, legacySQLiteFilename)
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("knowledgegraph: stat legacy sqlite: %w", err)
	}
	return current, nil
}

func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT NOT NULL PRIMARY KEY,
			node_type TEXT NOT NULL DEFAULT 'entity',
			aliases_json TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'active',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			embedding BLOB
		)`, tableNodes),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			graph_kind TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			polarity INTEGER NOT NULL,
			confidence REAL NOT NULL,
			condition_text TEXT NOT NULL DEFAULT '',
			source_type TEXT NOT NULL DEFAULT '',
			source_ref TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			observed_at DATETIME,
			valid_from DATETIME,
			valid_until DATETIME,
			evidence_count INTEGER NOT NULL DEFAULT 0,
			failed_count INTEGER NOT NULL DEFAULT 0,
			last_verified_at DATETIME,
			decay_half_life_days INTEGER NOT NULL DEFAULT 30,
			expires_at DATETIME,
			is_executable INTEGER NOT NULL DEFAULT 0,
			activation_rule TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (from_id, to_id, graph_kind, relation_type, condition_text),
			FOREIGN KEY (from_id) REFERENCES %s(id),
			FOREIGN KEY (to_id) REFERENCES %s(id)
		)`, tableEdges, tableNodes, tableNodes),
		`CREATE TABLE IF NOT EXISTS kg_edge_evidence (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			edge_id INTEGER NOT NULL,
			source_type TEXT NOT NULL DEFAULT '',
			source_ref TEXT NOT NULL DEFAULT '',
			snippet TEXT NOT NULL DEFAULT '',
			observed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			supports INTEGER NOT NULL,
			weight REAL NOT NULL DEFAULT 1.0,
			FOREIGN KEY (edge_id) REFERENCES kg_edges(id) ON DELETE CASCADE
		)`,
	}
	for _, q := range stmts {
		if _, err := s.conn.Exec(q); err != nil {
			return fmt.Errorf("knowledgegraph: migrate: %w", err)
		}
	}
	return nil
}

func (s *Store) NodeExists(id string) (bool, error) {
	var n int
	err := s.conn.QueryRow(fmt.Sprintf(`SELECT 1 FROM %s WHERE id = ? LIMIT 1`, tableNodes), id).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) NodeUpsert(id, nodeType, aliasesJSON, status string, embedding []byte) error {
	if id == "" {
		return fmt.Errorf("knowledgegraph: upsert node: empty id")
	}
	if strings.TrimSpace(nodeType) == "" {
		nodeType = "entity"
	}
	if strings.TrimSpace(status) == "" {
		status = "active"
	}
	if strings.TrimSpace(aliasesJSON) == "" {
		aliasesJSON = "[]"
	}
	_, err := s.conn.Exec(fmt.Sprintf(`
		INSERT INTO %s (id, node_type, aliases_json, status, embedding)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			node_type = excluded.node_type,
			aliases_json = excluded.aliases_json,
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP,
			embedding = COALESCE(excluded.embedding, %s.embedding)
	`, tableNodes, tableNodes), id, nodeType, aliasesJSON, status, embedding)
	return err
}

func (s *Store) NodesSelectAll() ([]NodeRow, error) {
	rows, err := s.conn.Query(fmt.Sprintf(`SELECT id, node_type, aliases_json, status, updated_at, embedding FROM %s`, tableNodes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRow
	for rows.Next() {
		var r NodeRow
		if err := rows.Scan(&r.ID, &r.NodeType, &r.AliasesJSON, &r.Status, &r.UpdatedAt, &r.Embedding); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) EdgeUpsert(in EdgeInput) (int64, error) {
	if in.FromID == "" || in.ToID == "" {
		return 0, fmt.Errorf("knowledgegraph: upsert edge: empty from or to")
	}
	if strings.TrimSpace(in.GraphKind) == "" {
		return 0, fmt.Errorf("knowledgegraph: upsert edge: graph_kind is required")
	}
	if strings.TrimSpace(in.RelationType) == "" {
		return 0, fmt.Errorf("knowledgegraph: upsert edge: relation_type is required")
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return 0, fmt.Errorf("knowledgegraph: upsert edge: confidence must be in [0,1]")
	}
	if in.DecayHalfLifeDays <= 0 {
		in.DecayHalfLifeDays = 30
	}
	isExec := 0
	if in.IsExecutable {
		isExec = 1
	}
	_, err := s.conn.Exec(fmt.Sprintf(`
		INSERT INTO %s (
			from_id, to_id, graph_kind, relation_type, polarity, confidence, condition_text,
			source_type, source_ref, observed_at, valid_from, valid_until,
			decay_half_life_days, expires_at, is_executable, activation_rule
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(from_id, to_id, graph_kind, relation_type, condition_text) DO UPDATE SET
			polarity = excluded.polarity,
			confidence = excluded.confidence,
			source_type = excluded.source_type,
			source_ref = excluded.source_ref,
			observed_at = excluded.observed_at,
			valid_from = excluded.valid_from,
			valid_until = excluded.valid_until,
			decay_half_life_days = excluded.decay_half_life_days,
			expires_at = excluded.expires_at,
			is_executable = excluded.is_executable,
			activation_rule = excluded.activation_rule,
			updated_at = CURRENT_TIMESTAMP
	`, tableEdges), in.FromID, in.ToID, in.GraphKind, in.RelationType, in.Polarity, in.Confidence, in.ConditionText,
		in.SourceType, in.SourceRef, in.ObservedAt, in.ValidFrom, in.ValidUntil,
		in.DecayHalfLifeDays, in.ExpiresAt, isExec, in.ActivationRule)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := s.conn.QueryRow(fmt.Sprintf(`
		SELECT id FROM %s
		WHERE from_id = ? AND to_id = ? AND graph_kind = ? AND relation_type = ? AND condition_text = ?
		LIMIT 1
	`, tableEdges), in.FromID, in.ToID, in.GraphKind, in.RelationType, in.ConditionText).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) EdgeEvidenceInsert(in EdgeEvidenceInput) error {
	if in.EdgeID <= 0 {
		return fmt.Errorf("knowledgegraph: edge evidence: edge_id must be > 0")
	}
	observedAt := time.Now().UTC()
	if in.ObservedAt != nil {
		observedAt = in.ObservedAt.UTC()
	}
	supports := 0
	if in.Supports {
		supports = 1
	}
	if in.Weight <= 0 {
		in.Weight = 1
	}
	_, err := s.conn.Exec(`
		INSERT INTO kg_edge_evidence (edge_id, source_type, source_ref, snippet, observed_at, supports, weight)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, in.EdgeID, in.SourceType, in.SourceRef, in.Snippet, observedAt, supports, in.Weight)
	if err != nil {
		return err
	}
	if supports == 1 {
		_, err = s.conn.Exec(fmt.Sprintf(`UPDATE %s SET evidence_count = evidence_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, tableEdges), in.EdgeID)
		return err
	}
	_, err = s.conn.Exec(fmt.Sprintf(`UPDATE %s SET failed_count = failed_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, tableEdges), in.EdgeID)
	return err
}

func (s *Store) EdgeVerify(edgeID int64, success bool, confidence *float64, verifiedAt time.Time) error {
	if edgeID <= 0 {
		return fmt.Errorf("knowledgegraph: verify edge: edge_id must be > 0")
	}
	baseSQL := fmt.Sprintf(`
		UPDATE %s
		SET evidence_count = evidence_count + ?,
			failed_count = failed_count + ?,
			last_verified_at = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, tableEdges)
	ev, fail := 0, 0
	if success {
		ev = 1
	} else {
		fail = 1
	}
	if _, err := s.conn.Exec(baseSQL, ev, fail, verifiedAt.UTC(), edgeID); err != nil {
		return err
	}
	if confidence != nil {
		if *confidence < 0 || *confidence > 1 {
			return fmt.Errorf("knowledgegraph: verify edge: confidence must be in [0,1]")
		}
		if _, err := s.conn.Exec(fmt.Sprintf(`UPDATE %s SET confidence = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, tableEdges), *confidence, edgeID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EdgesSelectAll() ([]EdgeRow, error) {
	rows, err := s.conn.Query(fmt.Sprintf(`
		SELECT id, from_id, to_id, graph_kind, relation_type, polarity, confidence, condition_text,
		       source_type, source_ref, created_at, observed_at, valid_from, valid_until,
			   evidence_count, failed_count, last_verified_at,
			   decay_half_life_days, expires_at, is_executable, activation_rule, updated_at
		FROM %s
	`, tableEdges))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgeRow
	for rows.Next() {
		var r EdgeRow
		var observedAt sql.NullTime
		var validFrom sql.NullTime
		var validUntil sql.NullTime
		var lastVerifiedAt sql.NullTime
		var expiresAt sql.NullTime
		var isExecutable int
		if err := rows.Scan(
			&r.ID, &r.FromID, &r.ToID, &r.GraphKind, &r.RelationType, &r.Polarity, &r.Confidence, &r.ConditionText,
			&r.SourceType, &r.SourceRef, &r.CreatedAt, &observedAt, &validFrom, &validUntil,
			&r.EvidenceCount, &r.FailedCount, &lastVerifiedAt,
			&r.DecayHalfLifeDays, &expiresAt, &isExecutable, &r.ActivationRule, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if observedAt.Valid {
			t := observedAt.Time
			r.ObservedAt = &t
		}
		if validFrom.Valid {
			t := validFrom.Time
			r.ValidFrom = &t
		}
		if validUntil.Valid {
			t := validUntil.Time
			r.ValidUntil = &t
		}
		if lastVerifiedAt.Valid {
			t := lastVerifiedAt.Time
			r.LastVerifiedAt = &t
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			r.ExpiresAt = &t
		}
		r.IsExecutable = isExecutable != 0
		out = append(out, r)
	}
	return out, rows.Err()
}
