package cli

import (
	"database/sql"
	"fmt"
	"time"
)

type microtone struct {
	Name     string
	Question string
	Position int
}

type subtone struct {
	Name          string
	Description   string
	Position      int
	TestCondition string
	TestConditions []string
	TestCommand   string
	DependsOn     string
	TestOutput    string
	AgentNotes    string
}

type microtoneEdge struct {
	From     string
	To       string
	EdgeType string
}

func seedMicrotoneGraph(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_microtones`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ensureSession(db)
	}

	microtones := []microtone{
		{Name: "set-git-clean", Question: "Is the git clean?", Position: 1},
		{Name: "align-goal-subtone-names", Question: "Is the tone goal aligned with subtone names?", Position: 2},
		{Name: "review-all-subtones", Question: "Are any subtones too large (over ~20 minutes)?", Position: 3},
		{Name: "subtone-review", Question: "Is this subtone ready to move forward?", Position: 4},
		{Name: "subtone-run-test", Question: "Did the subtone test pass?", Position: 5},
		{Name: "subtone-review-complete", Question: "All subtones reviewed?", Position: 6},
		{Name: "start-complete-phase", Question: "Ready to start the complete phase?", Position: 7},
		{Name: "confirm-pr-merged", Question: "Is the PR merged?", Position: 8},
		{Name: "complete", Question: "Complete the tone?", Position: 9},
	}
	edges := []microtoneEdge{
		{From: "set-git-clean", To: "align-goal-subtone-names", EdgeType: "linear"},
		{From: "align-goal-subtone-names", To: "review-all-subtones", EdgeType: "linear"},
		{From: "review-all-subtones", To: "subtone-review", EdgeType: "linear"},
		{From: "subtone-review", To: "subtone-run-test", EdgeType: "linear"},
		{From: "subtone-run-test", To: "subtone-review", EdgeType: "loop"},
		{From: "subtone-review-complete", To: "start-complete-phase", EdgeType: "linear"},
		{From: "start-complete-phase", To: "confirm-pr-merged", EdgeType: "linear"},
		{From: "confirm-pr-merged", To: "complete", EdgeType: "linear"},
	}

	subtones := []subtone{
		{Name: "alpha", Description: "First subtone", Position: 1},
		{Name: "beta", Description: "Second subtone", Position: 2},
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, mt := range microtones {
		if _, err := tx.Exec(
			`INSERT INTO nexttone_microtones (name, position, question) VALUES (?, ?, ?)`,
			mt.Name, mt.Position, mt.Question,
		); err != nil {
			return err
		}
	}
	for _, edge := range edges {
		if _, err := tx.Exec(
			`INSERT INTO nexttone_microtone_edges (from_name, to_name, edge_type) VALUES (?, ?, ?)`,
			edge.From, edge.To, edge.EdgeType,
		); err != nil {
			return err
		}
	}

	for _, st := range subtones {
		if _, err := tx.Exec(
			`INSERT INTO nexttone_subtones (name, position, description, test_condition, test_command, depends_on, test_output, agent_notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			st.Name,
			st.Position,
			st.Description,
			st.TestCondition,
			st.TestCommand,
			st.DependsOn,
			st.TestOutput,
			st.AgentNotes,
		); err != nil {
			return err
		}
	}

	if err := ensureSessionTx(tx, microtones[0].Name, subtones[0].Name); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureSession(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_sessions WHERE id = 'default'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var first string
	if err := db.QueryRow(`SELECT name FROM nexttone_microtones ORDER BY position LIMIT 1`).Scan(&first); err != nil {
		return err
	}
	var firstSubtone string
	if err := db.QueryRow(`SELECT name FROM nexttone_subtones ORDER BY position LIMIT 1`).Scan(&firstSubtone); err != nil {
		return err
	}
	_, err := db.Exec(
		`INSERT INTO nexttone_sessions (id, current_microtone, current_subtone, updated_at) VALUES ('default', ?, ?, ?)`,
		first, firstSubtone, time.Now().Format(time.RFC3339),
	)
	return err
}

func ensureSessionTx(tx *sql.Tx, first, firstSubtone string) error {
	_, err := tx.Exec(
		`INSERT INTO nexttone_sessions (id, current_microtone, current_subtone, updated_at) VALUES ('default', ?, ?, ?)`,
		first, firstSubtone, time.Now().Format(time.RFC3339),
	)
	return err
}

func getCurrentMicrotone(db *sql.DB) (microtone, string, error) {
	var current string
	var currentSubtone string
	if err := db.QueryRow(`SELECT current_microtone, current_subtone FROM nexttone_sessions WHERE id = 'default'`).Scan(&current, &currentSubtone); err != nil {
		return microtone{}, "", err
	}

	var mt microtone
	if err := db.QueryRow(
		`SELECT name, question, position FROM nexttone_microtones WHERE name = ?`,
		current,
	).Scan(&mt.Name, &mt.Question, &mt.Position); err != nil {
		return microtone{}, "", err
	}
	return mt, currentSubtone, nil
}

func getNextMicrotone(db *sql.DB, current string) (string, error) {
	var next string
	err := db.QueryRow(
		`SELECT to_name FROM nexttone_microtone_edges WHERE from_name = ? AND edge_type = 'linear' LIMIT 1`,
		current,
	).Scan(&next)
	if err == sql.ErrNoRows {
		return current, nil
	}
	if err != nil {
		return "", err
	}
	return next, nil
}

func advanceMicrotone(db *sql.DB, current string) (string, error) {
	next, err := getNextMicrotone(db, current)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(
		`UPDATE nexttone_sessions SET current_microtone = ?, updated_at = ? WHERE id = 'default'`,
		next, time.Now().Format(time.RFC3339),
	); err != nil {
		return "", err
	}
	return next, nil
}

func setCurrentMicrotone(db *sql.DB, next string) error {
	_, err := db.Exec(
		`UPDATE nexttone_sessions SET current_microtone = ?, updated_at = ? WHERE id = 'default'`,
		next, time.Now().Format(time.RFC3339),
	)
	return err
}

func getSubtones(db *sql.DB) ([]subtone, error) {
	subtones := []subtone{}
	rows, err := db.Query(`SELECT name, description, position, test_condition, test_command, depends_on, test_output, agent_notes
		FROM nexttone_subtones ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var st subtone
		var description sql.NullString
		var testCondition sql.NullString
		var testCommand sql.NullString
		var dependsOn sql.NullString
		var testOutput sql.NullString
		var agentNotes sql.NullString
		if err := rows.Scan(
			&st.Name,
			&description,
			&st.Position,
			&testCondition,
			&testCommand,
			&dependsOn,
			&testOutput,
			&agentNotes,
		); err != nil {
			return nil, err
		}
		st.Description = description.String
		st.TestCondition = testCondition.String
		st.TestCommand = testCommand.String
		st.DependsOn = dependsOn.String
		st.TestOutput = testOutput.String
		st.AgentNotes = agentNotes.String
		conditions, err := getSubtoneConditions(db, st.Name)
		if err != nil {
			return nil, err
		}
		st.TestConditions = conditions
		subtones = append(subtones, st)
	}
	return subtones, nil
}

func advanceSubtone(db *sql.DB, current string) (string, bool, error) {
	subtones, err := getSubtones(db)
	if err != nil {
		return "", false, err
	}
	if len(subtones) == 0 {
		return "", false, nil
	}
	for i, st := range subtones {
		if st.Name == current {
			next := subtones[(i+1)%len(subtones)]
			wrap := i+1 == len(subtones)
			if _, err := db.Exec(
				`UPDATE nexttone_sessions SET current_subtone = ?, updated_at = ? WHERE id = 'default'`,
				next.Name, time.Now().Format(time.RFC3339),
			); err != nil {
				return "", false, err
			}
			return next.Name, wrap, nil
		}
	}
	first := subtones[0]
	if _, err := db.Exec(
		`UPDATE nexttone_sessions SET current_subtone = ?, updated_at = ? WHERE id = 'default'`,
		first.Name, time.Now().Format(time.RFC3339),
	); err != nil {
		return "", false, err
	}
	return first.Name, false, nil
}

func recordSignature(db *sql.DB, microtoneName, response string) error {
	_, err := db.Exec(
		`INSERT INTO nexttone_signatures (microtone, response, timestamp) VALUES (?, ?, ?)`,
		microtoneName, response, time.Now().Format(time.RFC3339),
	)
	return err
}

func loadGraph(db *sql.DB) ([]microtone, []microtoneEdge, error) {
	nodes := []microtone{}
	rows, err := db.Query(`SELECT name, question, position FROM nexttone_microtones ORDER BY position`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mt microtone
		if err := rows.Scan(&mt.Name, &mt.Question, &mt.Position); err != nil {
			return nil, nil, err
		}
		nodes = append(nodes, mt)
	}

	edges := []microtoneEdge{}
	edgeRows, err := db.Query(`SELECT from_name, to_name, edge_type FROM nexttone_microtone_edges`)
	if err != nil {
		return nil, nil, err
	}
	defer edgeRows.Close()
	for edgeRows.Next() {
		var edge microtoneEdge
		if err := edgeRows.Scan(&edge.From, &edge.To, &edge.EdgeType); err != nil {
			return nil, nil, err
		}
		edges = append(edges, edge)
	}
	return nodes, edges, nil
}

func formatGraph(nodes []microtone, edges []microtoneEdge, current string, currentSubtone string, subtones []subtone) string {
	lines := []string{
		"MICROTONE GRAPH",
		fmt.Sprintf("CURRENT: %s", current),
		fmt.Sprintf("CURRENT SUBTONE: %s", currentSubtone),
		"",
	}
	for i, node := range nodes {
		prefix := "  "
		if node.Name == current {
			prefix = "* "
		}
		line := fmt.Sprintf("%s%s", prefix, node.Name)
		if i < len(nodes)-1 {
			line += " ->"
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	for _, edge := range edges {
		if edge.EdgeType == "loop" {
			lines = append(lines, fmt.Sprintf("LOOP: %s -> %s", edge.From, edge.To))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "SUBTONES:")
	for _, st := range subtones {
		marker := "  "
		if st.Name == currentSubtone {
			marker = "* "
		}
		lines = append(lines, fmt.Sprintf("%s%s", marker, st.Name))
	}
	return stringsJoin(lines)
}

func stringsJoin(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}
