package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

type tableRow struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

func main() {
	port := "8080"
	cwd, _ := os.Getwd()

	// Try a few locations for UI dist
	candidates := []string{
		filepath.Join(cwd, "ui", "dist"),
		filepath.Join(cwd, "src", "plugins", "dag", "src_v3", "ui", "dist"),
		filepath.Join(cwd, "plugins", "dag", "src_v3", "ui", "dist"),
		filepath.Join(cwd, "..", "ui", "dist"),
	}
	uiPath := ""
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			uiPath = c
			break
		}
	}
	if uiPath == "" {
		// Try one more if we are running from src
		if filepath.Base(cwd) == "src" {
			uiPath = filepath.Join(cwd, "plugins", "dag", "src_v3", "ui", "dist")
		} else {
			uiPath = filepath.Join(cwd, "ui", "dist")
		}
	}

	dbPath := resolveDBPath(cwd)
	logPath := resolveTestLogPath(cwd)

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	http.HandleFunc("/api/dag-table", func(w http.ResponseWriter, _ *http.Request) {
		rows, err := queryDagTableRows(dbPath)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows})
	})
	http.HandleFunc("/api/test-log", func(w http.ResponseWriter, r *http.Request) {
		offset := int64(0)
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed >= 0 {
				offset = parsed
			}
		}
		nextOffset, lines, err := readLogDelta(logPath, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"offset": nextOffset,
			"lines":  lines,
		})
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rel := r.URL.Path
		if len(rel) > 0 && rel[0] == '/' {
			rel = rel[1:]
		}
		path := filepath.Join(uiPath, rel)
		if r.URL.Path == "/" {
			path = filepath.Join(uiPath, "index.html")
		}
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(uiPath, "index.html")
		}
		http.ServeFile(w, r, path)
	})

	fmt.Printf("DAG Server starting on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}

func resolveDBPath(cwd string) string {
	candidates := []string{
		filepath.Join(cwd, "test", "test.duckdb"),
		filepath.Join(cwd, "plugins", "dag", "src_v3", "test", "test.duckdb"),
		filepath.Join(cwd, "src", "plugins", "dag", "src_v3", "test", "test.duckdb"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// Try relative from cmd
	if base := filepath.Dir(cwd); strings.HasSuffix(base, "src_v3") {
		return filepath.Join(base, "test", "test.duckdb")
	}
	return filepath.Join(cwd, "test.duckdb")
}

func resolveTestLogPath(cwd string) string {
	candidates := []string{
		filepath.Join(cwd, "test", "test.log"),
		filepath.Join(cwd, "plugins", "dag", "src_v3", "test", "test.log"),
		filepath.Join(cwd, "src", "plugins", "dag", "src_v3", "test", "test.log"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// Try relative from cmd
	if base := filepath.Dir(cwd); strings.HasSuffix(base, "src_v3") {
		return filepath.Join(base, "test", "test.log")
	}
	return filepath.Join(cwd, "test.log")
}

func readLogDelta(path string, offset int64) (int64, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, []string{}, nil
		}
		return offset, nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return offset, nil, err
	}
	size := info.Size()
	if offset > size {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return offset, nil, err
	}
	next := offset + int64(len(data))
	if len(data) == 0 {
		return next, []string{}, nil
	}
	chunks := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if c == "" {
			continue
		}
		lines = append(lines, c)
	}
	return next, lines, nil
}

func queryDagTableRows(dbPath string) ([]tableRow, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	defer db.Close()

	if err := ensureDuckPGQLoaded(db); err != nil {
		return nil, err
	}

	statements := []string{
		`CREATE OR REPLACE PROPERTY GRAPH dag_pg
			VERTEX TABLES (
				dag_node
			)
			EDGE TABLES (
				dag_edge
					SOURCE KEY (from_node_id) REFERENCES dag_node (node_id)
					DESTINATION KEY (to_node_id) REFERENCES dag_node (node_id)
			);`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("duckdb setup failed: %w", err)
		}
	}

	rows := make([]tableRow, 0, 8)
	appendMetric := func(key, query string) error {
		var value any
		if err := db.QueryRow(query).Scan(&value); err != nil {
			return err
		}
		rows = append(rows, tableRow{Key: key, Value: fmt.Sprint(value), Status: "OK"})
		return nil
	}

	if err := appendMetric("node_count", `SELECT COUNT(*) FROM dag_node;`); err != nil {
		return nil, err
	}
	if err := appendMetric("edge_count", `SELECT COUNT(*) FROM dag_edge;`); err != nil {
		return nil, err
	}
	if err := appendMetric("layer_count", `SELECT COUNT(*) FROM dag_layer;`); err != nil {
		return nil, err
	}
	if err := appendMetric("graph_edge_match_count", `
		SELECT COUNT(*)
		FROM GRAPH_TABLE (dag_pg
			MATCH (a:dag_node)-[e:dag_edge]->(b:dag_node)
			COLUMNS (a.node_id AS src, b.node_id AS dst)
		);
	`); err != nil {
		return nil, err
	}
	if err := appendMetric("shortest_path_hops_root_to_leaf", `
		SELECT hops
		FROM GRAPH_TABLE (dag_pg
			MATCH p = ANY SHORTEST (a:dag_node)-[e:dag_edge]->+(b:dag_node)
			WHERE a.node_id = 'n_root' AND b.node_id = 'n_leaf'
			COLUMNS (path_length(p) AS hops)
		)
		LIMIT 1;
	`); err != nil {
		return nil, err
	}
	if err := appendMetric("rank_violation_count", `
		SELECT COUNT(*)
		FROM dag_edge e
		JOIN dag_node n_from ON n_from.node_id = e.from_node_id
		JOIN dag_node n_to ON n_to.node_id = e.to_node_id
		WHERE n_to.rank <= n_from.rank;
	`); err != nil {
		return nil, err
	}

	return rows, nil
}

func ensureDuckPGQLoaded(db *sql.DB) error {
	if _, err := db.Exec(`LOAD duckpgq;`); err == nil {
		return nil
	}
	if _, err := db.Exec(`INSTALL duckpgq FROM community;`); err != nil {
		return fmt.Errorf("duckpgq install failed: %w", err)
	}
	if _, err := db.Exec(`LOAD duckpgq;`); err != nil {
		return fmt.Errorf("duckpgq load failed: %w", err)
	}
	return nil
}
