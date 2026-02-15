package main

import (
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb"
)

func Run01DuckDBGraphQueries() error {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return fmt.Errorf("open duckdb: %w", err)
	}
	defer db.Close()

	statements := []string{
		`INSTALL duckpgq FROM community;`,
		`LOAD duckpgq;`,
		`CREATE TABLE dag_graph (graph_id VARCHAR, root_layer_id VARCHAR);`,
		`CREATE TABLE dag_layer (layer_id VARCHAR, graph_id VARCHAR, parent_node_id VARCHAR, depth INTEGER);`,
		`CREATE TABLE dag_node (node_id VARCHAR, layer_id VARCHAR, label VARCHAR, rank INTEGER);`,
		`CREATE TABLE dag_edge (edge_id VARCHAR, layer_id VARCHAR, from_node_id VARCHAR, to_node_id VARCHAR, weight DOUBLE);`,
		`INSERT INTO dag_graph VALUES ('g1', 'root');`,
		`INSERT INTO dag_layer VALUES ('root', 'g1', NULL, 0);`,
		`INSERT INTO dag_node VALUES
			('n_root', 'root', 'Root', 0),
			('n_mid_a', 'root', 'Mid A', 1),
			('n_mid_b', 'root', 'Mid B', 1),
			('n_leaf', 'root', 'Leaf', 2);`,
		`INSERT INTO dag_edge VALUES
			('e1', 'root', 'n_root', 'n_mid_a', 0.9),
			('e2', 'root', 'n_mid_a', 'n_leaf', 0.8),
			('e3', 'root', 'n_root', 'n_mid_b', 0.7),
			('e4', 'root', 'n_mid_b', 'n_leaf', 0.6);`,
		`CREATE PROPERTY GRAPH dag_pg
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
			return fmt.Errorf("duckdb statement failed: %s: %w", stmt, err)
		}
	}

	var edgeCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM GRAPH_TABLE (dag_pg
			MATCH (a:dag_node)-[e:dag_edge]->(b:dag_node)
			COLUMNS (a.node_id AS src, b.node_id AS dst)
		);
	`).Scan(&edgeCount); err != nil {
		return fmt.Errorf("graph edge match query failed: %w", err)
	}
	if edgeCount != 4 {
		return fmt.Errorf("expected 4 graph edges from GRAPH_TABLE, got %d", edgeCount)
	}

	var hops int
	if err := db.QueryRow(`
		SELECT hops
		FROM GRAPH_TABLE (dag_pg
			MATCH p = ANY SHORTEST (a:dag_node)-[e:dag_edge]->+(b:dag_node)
			WHERE a.node_id = 'n_root' AND b.node_id = 'n_leaf'
			COLUMNS (path_length(p) AS hops)
		)
		LIMIT 1;
	`).Scan(&hops); err != nil {
		return fmt.Errorf("shortest-path query failed: %w", err)
	}
	if hops != 2 {
		return fmt.Errorf("expected shortest path hops=2, got %d", hops)
	}

	var rankViolations int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM dag_edge e
		JOIN dag_node n_from ON n_from.node_id = e.from_node_id
		JOIN dag_node n_to ON n_to.node_id = e.to_node_id
		WHERE n_to.rank <= n_from.rank;
	`).Scan(&rankViolations); err != nil {
		return fmt.Errorf("rank validation query failed: %w", err)
	}
	if rankViolations != 0 {
		return fmt.Errorf("expected 0 rank violations, got %d", rankViolations)
	}

	return nil
}
