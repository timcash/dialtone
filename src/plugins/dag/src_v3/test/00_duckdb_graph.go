package test

import (
	"database/sql"
	"fmt"
	"sort"

	_ "github.com/marcboeker/go-duckdb"
)

func Run01DuckDBGraphQueries(ctx *testCtx) (string, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return "", fmt.Errorf("open duckdb: %w", err)
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
		`INSERT INTO dag_layer VALUES
			('root', 'g1', NULL, 0),
			('nested_a', 'g1', 'n_mid_a', 1),
			('nested_b', 'g1', 'n_mid_a', 1);`,
		`INSERT INTO dag_node VALUES
			('n_root', 'root', 'Root', 0),
			('n_mid_a', 'root', 'Mid A', 1),
			('n_mid_b', 'root', 'Mid B', 1),
			('n_leaf', 'root', 'Leaf', 2),
			('n_nested_1', 'nested_a', 'Nested 1', 2),
			('n_nested_2', 'nested_a', 'Nested 2', 3),
			('n_nested_3', 'nested_b', 'Nested 3', 2);`,
		`INSERT INTO dag_edge VALUES
			('e1', 'root', 'n_root', 'n_mid_a', 0.9),
			('e2', 'root', 'n_mid_a', 'n_leaf', 0.8),
			('e3', 'root', 'n_root', 'n_mid_b', 0.7),
			('e4', 'root', 'n_mid_b', 'n_leaf', 0.6),
			('e5', 'nested_a', 'n_mid_a', 'n_nested_1', 0.5),
			('e6', 'nested_a', 'n_nested_1', 'n_nested_2', 0.4),
			('e7', 'nested_b', 'n_mid_a', 'n_nested_3', 0.3);`,
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
			return "", fmt.Errorf("duckdb statement failed: %s: %w", stmt, err)
		}
	}

	var edgeCount int
	ctx.logf("LOOKING FOR: graph_edge_match_count")
	ctx.logf("[GRAPH] running: graph_edge_match_count")
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM GRAPH_TABLE (dag_pg
			MATCH (a:dag_node)-[e:dag_edge]->(b:dag_node)
			COLUMNS (a.node_id AS src, b.node_id AS dst)
		);
	`).Scan(&edgeCount); err != nil {
		return "", fmt.Errorf("graph edge match query failed: %w", err)
	}
	if edgeCount != 7 {
		return "", fmt.Errorf("expected 7 graph edges from GRAPH_TABLE, got %d", edgeCount)
	}

	var hops int
	ctx.logf("LOOKING FOR: shortest_path_hops_root_to_leaf")
	ctx.logf("[GRAPH] running: shortest_path_hops_root_to_leaf")
	if err := db.QueryRow(`
		SELECT hops
		FROM GRAPH_TABLE (dag_pg
			MATCH p = ANY SHORTEST (a:dag_node)-[e:dag_edge]->+(b:dag_node)
			WHERE a.node_id = 'n_root' AND b.node_id = 'n_leaf'
			COLUMNS (path_length(p) AS hops)
		)
		LIMIT 1;
	`).Scan(&hops); err != nil {
		return "", fmt.Errorf("shortest-path query failed: %w", err)
	}
	if hops != 2 {
		return "", fmt.Errorf("expected shortest path hops=2, got %d", hops)
	}

	var rankViolations int
	ctx.logf("LOOKING FOR: rank_violation_count")
	ctx.logf("[GRAPH] running: rank_violation_count")
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM dag_edge e
		JOIN dag_node n_from ON n_from.node_id = e.from_node_id
		JOIN dag_node n_to ON n_to.node_id = e.to_node_id
		WHERE n_to.rank <= n_from.rank;
	`).Scan(&rankViolations); err != nil {
		return "", fmt.Errorf("rank validation query failed: %w", err)
	}
	if rankViolations != 0 {
		return "", fmt.Errorf("expected 0 rank violations, got %d", rankViolations)
	}

	// UI action: given a node, find all nested nodes (nodes inside its nested layers, recursively).
	ctx.logf("LOOKING FOR: nested_nodes_for_n_mid_a")
	ctx.logf("[GRAPH] running: nested_nodes_for_n_mid_a")
	nestedNodes, err := queryStringList(db, `
		WITH RECURSIVE nested_layers(layer_id) AS (
			SELECT l.layer_id
			FROM dag_layer l
			WHERE l.parent_node_id = 'n_mid_a'
			UNION ALL
			SELECT l2.layer_id
			FROM dag_layer l2
			JOIN nested_layers nl ON l2.parent_node_id IN (
				SELECT n.node_id FROM dag_node n WHERE n.layer_id = nl.layer_id
			)
		)
		SELECT n.node_id
		FROM dag_node n
		JOIN nested_layers nl ON nl.layer_id = n.layer_id
		ORDER BY n.node_id;
	`)
	if err != nil {
		return "", fmt.Errorf("nested-node query failed: %w", err)
	}
	if err := assertStringSetEquals("nested nodes for n_mid_a", nestedNodes, []string{"n_nested_1", "n_nested_2", "n_nested_3"}); err != nil {
		return "", err
	}

	// UI action: given a node, find all input nodes (edges pointing into the node).
	ctx.logf("LOOKING FOR: input_nodes_for_n_leaf")
	ctx.logf("[GRAPH] running: input_nodes_for_n_leaf")
	inputNodes, err := queryStringList(db, `
		SELECT input_node
		FROM GRAPH_TABLE (dag_pg
			MATCH (src:dag_node)-[e:dag_edge]->(dst:dag_node)
			WHERE dst.node_id = 'n_leaf'
			COLUMNS (src.node_id AS input_node)
		)
		ORDER BY input_node;
	`)
	if err != nil {
		return "", fmt.Errorf("input-node query failed: %w", err)
	}
	if err := assertStringSetEquals("input nodes for n_leaf", inputNodes, []string{"n_mid_a", "n_mid_b"}); err != nil {
		return "", err
	}

	// UI action: given a node, find all output nodes (edges pointing out of the node).
	ctx.logf("LOOKING FOR: output_nodes_for_n_root")
	ctx.logf("[GRAPH] running: output_nodes_for_n_root")
	outputNodes, err := queryStringList(db, `
		SELECT output_node
		FROM GRAPH_TABLE (dag_pg
			MATCH (src:dag_node)-[e:dag_edge]->(dst:dag_node)
			WHERE src.node_id = 'n_root'
			COLUMNS (dst.node_id AS output_node)
		)
		ORDER BY output_node;
	`)
	if err != nil {
		return "", fmt.Errorf("output-node query failed: %w", err)
	}
	if err := assertStringSetEquals("output nodes for n_root", outputNodes, []string{"n_mid_a", "n_mid_b"}); err != nil {
		return "", err
	}

	return "Validated core DAG graph queries in DuckDB/duckpgq for edge count, shortest path, rank rules, and input/output nested-node derivations.", nil
}

func queryStringList(db *sql.DB, query string) ([]string, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func assertStringSetEquals(label string, got, want []string) error {
	gotSorted := append([]string(nil), got...)
	wantSorted := append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if len(gotSorted) != len(wantSorted) {
		return fmt.Errorf("%s mismatch: got %v, want %v", label, gotSorted, wantSorted)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			return fmt.Errorf("%s mismatch: got %v, want %v", label, gotSorted, wantSorted)
		}
	}
	return nil
}
