package knowledgegraph

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	sqliteFilename       = "knowledgegraph.sqlite"
	legacySQLiteFilename = "octosucker.sqlite"
	tableNodes           = "kg_nodes"
	tableEdges           = "kg_edges"
)

type NodeRow struct {
	ID        string `json:"id"`
	Embedding []byte `json:"-"`
}

type EdgeRow struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Positive bool   `json:"positive"`
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
		if cerr := sqlDB.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close sqlite after ping failure: %w", cerr))
		}
		return nil, fmt.Errorf("knowledgegraph: ping sqlite: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		if cerr := sqlDB.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close sqlite after pragma failure: %w", cerr))
		}
		return nil, fmt.Errorf("knowledgegraph: pragma journal_mode: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		if cerr := sqlDB.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close sqlite after pragma failure: %w", cerr))
		}
		return nil, fmt.Errorf("knowledgegraph: pragma busy_timeout: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		if cerr := sqlDB.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close sqlite after pragma failure: %w", cerr))
		}
		return nil, fmt.Errorf("knowledgegraph: pragma foreign_keys: %w", err)
	}
	s := &Store{conn: sqlDB}
	if err := s.migrate(); err != nil {
		if cerr := sqlDB.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close sqlite after migration failure: %w", cerr))
		}
		return nil, err
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
			embedding BLOB
		)`, tableNodes),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			positive INTEGER NOT NULL,
			PRIMARY KEY (from_id, to_id),
			FOREIGN KEY (from_id) REFERENCES %s(id),
			FOREIGN KEY (to_id) REFERENCES %s(id)
		)`, tableEdges, tableNodes, tableNodes),
	}
	for _, q := range stmts {
		if _, err := s.conn.Exec(q); err != nil {
			return fmt.Errorf("knowledgegraph: migrate: %w", err)
		}
	}
	if err := s.migrateKnowledgeGraphEdgesEndpointColumns(); err != nil {
		return err
	}
	return s.migrateKnowledgeGraphNodeEmbeddingColumn()
}

func (s *Store) migrateKnowledgeGraphEdgesEndpointColumns() error {
	var fromIDCols int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info(%q) WHERE name = 'from_id'`, tableEdges)
	if err := s.conn.QueryRow(q).Scan(&fromIDCols); err != nil {
		return fmt.Errorf("knowledgegraph: migrate kg_edges pragma from_id: %w", err)
	}
	if fromIDCols > 0 {
		return nil
	}
	var oldFrom int
	q2 := fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info(%q) WHERE name = 'from'`, tableEdges)
	if err := s.conn.QueryRow(q2).Scan(&oldFrom); err != nil {
		return fmt.Errorf("knowledgegraph: migrate kg_edges pragma from: %w", err)
	}
	if oldFrom == 0 {
		return nil
	}
	if _, err := s.conn.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN "from" TO from_id`, tableEdges)); err != nil {
		return fmt.Errorf("knowledgegraph: migrate kg_edges rename from: %w", err)
	}
	if _, err := s.conn.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN "to" TO to_id`, tableEdges)); err != nil {
		return fmt.Errorf("knowledgegraph: migrate kg_edges rename to: %w", err)
	}
	return nil
}

func (s *Store) migrateKnowledgeGraphNodeEmbeddingColumn() error {
	var cnt int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info(%q) WHERE name = 'embedding'`, tableNodes)
	if err := s.conn.QueryRow(q).Scan(&cnt); err != nil {
		return fmt.Errorf("knowledgegraph: migrate kg_node embedding pragma: %w", err)
	}
	if cnt > 0 {
		return nil
	}
	if _, err := s.conn.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN embedding BLOB`, tableNodes)); err != nil {
		return fmt.Errorf("knowledgegraph: migrate kg_node add embedding: %w", err)
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

func (s *Store) NodeInsert(id string, embedding []byte) error {
	if id == "" {
		return fmt.Errorf("knowledgegraph: insert node: empty id")
	}
	_, err := s.conn.Exec(fmt.Sprintf(`INSERT INTO %s (id, embedding) VALUES (?, ?)`, tableNodes), id, embedding)
	return err
}

func (s *Store) NodesSelectAll() ([]NodeRow, error) {
	rows, err := s.conn.Query(fmt.Sprintf(`SELECT id, embedding FROM %s`, tableNodes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRow
	for rows.Next() {
		var r NodeRow
		if err := rows.Scan(&r.ID, &r.Embedding); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) EdgeExists(fromID, toID string) (bool, error) {
	var n int
	err := s.conn.QueryRow(fmt.Sprintf(`SELECT 1 FROM %s WHERE from_id = ? AND to_id = ? LIMIT 1`, tableEdges), fromID, toID).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) EdgeInsert(fromID, toID string, positive bool) error {
	if fromID == "" || toID == "" {
		return fmt.Errorf("knowledgegraph: insert edge: empty from or to")
	}
	p := 0
	if positive {
		p = 1
	}
	_, err := s.conn.Exec(fmt.Sprintf(`INSERT INTO %s (from_id, to_id, positive) VALUES (?, ?, ?)`, tableEdges), fromID, toID, p)
	return err
}

func (s *Store) EdgesSelectAll() ([]EdgeRow, error) {
	rows, err := s.conn.Query(fmt.Sprintf(`SELECT from_id, to_id, positive FROM %s`, tableEdges))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgeRow
	for rows.Next() {
		var r EdgeRow
		var pos int
		if err := rows.Scan(&r.FromID, &r.ToID, &pos); err != nil {
			return nil, err
		}
		r.Positive = pos != 0
		out = append(out, r)
	}
	return out, rows.Err()
}
