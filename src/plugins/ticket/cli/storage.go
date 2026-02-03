//go:build !no_duckdb

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

var ErrTicketNotFound = errors.New("ticket not found")

const (
	keysDBFilename = "keys.duckdb"

	// V2 storage:
	// Each ticket stores its own DuckDB at:
	//   src/tickets/<ticket-id>/<ticket-id>.duckdb
	currentTicketFilename     = ".current_ticket"
	currentTicketModeFilename = ".current_ticket_mode"
	ticketDBExtension         = ".duckdb"
)

func ticketDir(ticketID string) string {
	return filepath.Join("src", "tickets", ticketID)
}

func currentTicketPath() string {
	return filepath.Join("src", "tickets", currentTicketFilename)
}

func currentTicketModePath() string {
	return filepath.Join("src", "tickets", currentTicketModeFilename)
}

func ticketDBPathFor(ticketID string) string {
	// Keep env override for integration tests / isolation, but default to per-ticket DB.
	if p := os.Getenv("TICKET_DB_PATH"); p != "" {
		return p
	}
	return filepath.Join(ticketDir(ticketID), ticketID+ticketDBExtension)
}

func keysDBPath() string {
	return keysDBFilename // Root of the repo
}

func openTicketDB(ticketID string) (*sql.DB, error) {
	if strings.TrimSpace(ticketID) == "" {
		return nil, fmt.Errorf("ticket ID is empty")
	}
	if err := os.MkdirAll(ticketDir(ticketID), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("duckdb", ticketDBPathFor(ticketID))
	if err != nil {
		return nil, err
	}
	if err := ensureTicketSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func openKeysDB() (*sql.DB, error) {
	db, err := sql.Open("duckdb", keysDBPath())
	if err != nil {
		return nil, err
	}
	if err := ensureKeysSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureTicketSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tickets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			tags TEXT,
			description TEXT,
			state TEXT,
			agent_summary TEXT,
			start_time TEXT,
			last_summary_time TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS ticket_summaries (
			ticket_id TEXT NOT NULL,
			subtask_name TEXT,
			timestamp TEXT NOT NULL,
			content TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS subtasks (
			ticket_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			name TEXT NOT NULL,
			tags TEXT,
			dependencies TEXT,
			description TEXT,
			test_conditions TEXT,
			test_command TEXT,
			agent_notes TEXT,
			reviewed_timestamp TEXT,
			pass_timestamp TEXT,
			fail_timestamp TEXT,
			status TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS ticket_logs (
			ticket_id TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			entry_type TEXT NOT NULL,
			message TEXT NOT NULL,
			subtask TEXT
		);`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	// Migrations for V2 summary fields
	migrations := []string{
		`ALTER TABLE tickets ADD COLUMN IF NOT EXISTS agent_summary TEXT;`,
		`ALTER TABLE tickets ADD COLUMN IF NOT EXISTS state TEXT;`,
		`ALTER TABLE tickets ADD COLUMN IF NOT EXISTS start_time TEXT;`,
		`ALTER TABLE tickets ADD COLUMN IF NOT EXISTS last_summary_time TEXT;`,
		`ALTER TABLE subtasks ADD COLUMN IF NOT EXISTS test_command TEXT;`,
		`ALTER TABLE subtasks ADD COLUMN IF NOT EXISTS reviewed_timestamp TEXT;`,
	}
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			// Ignore errors for already existing columns if IF NOT EXISTS isn't supported/needed
		}
	}

	return nil
}

func ensureKeysSchema(db *sql.DB) error {
	stmt := `CREATE TABLE IF NOT EXISTS keys (
		name TEXT PRIMARY KEY,
		encrypted_value BLOB,
		salt BLOB,
		nonce BLOB
	);`
	_, err := db.Exec(stmt)
	return err
}

func GetTicket(ticketID string) (*Ticket, error) {
	if ticketID == "" {
		return nil, fmt.Errorf("ticket ID is empty")
	}
	db, err := openTicketDB(ticketID)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var id string
	var name string
	var tags sql.NullString
	var description sql.NullString
	var state sql.NullString
	var agentSummary sql.NullString
	var startTime sql.NullString
	var lastSummaryTime sql.NullString
	row := db.QueryRow(`SELECT id, name, tags, description, state, agent_summary, start_time, last_summary_time FROM tickets WHERE id = ?`, ticketID)
	if err := row.Scan(&id, &name, &tags, &description, &state, &agentSummary, &startTime, &lastSummaryTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}

	ticket := &Ticket{
		ID:              id,
		Name:            name,
		Description:     description.String,
		State:           state.String,
		AgentSummary:    agentSummary.String,
		StartTime:       startTime.String,
		LastSummaryTime: lastSummaryTime.String,
	}
	if strings.TrimSpace(ticket.State) == "" {
		ticket.State = "new"
	}
	if tagValues, err := decodeStringSlice(tags); err != nil {
		return nil, err
	} else if len(tagValues) > 0 {
		ticket.Tags = tagValues
	}

	rows, err := db.Query(`SELECT name, tags, dependencies, description, test_conditions, test_command, agent_notes, reviewed_timestamp, pass_timestamp, fail_timestamp, status
		FROM subtasks WHERE ticket_id = ? ORDER BY position`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var st Subtask
		var stTags sql.NullString
		var stDeps sql.NullString
		var stTests sql.NullString
		var stTestCommand sql.NullString
		var stNotes sql.NullString
		var stReviewed sql.NullString
		var stPass sql.NullString
		var stFail sql.NullString
		var stStatus sql.NullString
		if err := rows.Scan(&st.Name, &stTags, &stDeps, &st.Description, &stTests, &stTestCommand, &stNotes, &stReviewed, &stPass, &stFail, &stStatus); err != nil {
			return nil, err
		}
		if tagValues, err := decodeStringSlice(stTags); err != nil {
			return nil, err
		} else {
			st.Tags = tagValues
		}
		if depValues, err := decodeStringSlice(stDeps); err != nil {
			return nil, err
		} else {
			st.Dependencies = depValues
		}
		if testValues, err := decodeTestConditions(stTests); err != nil {
			return nil, err
		} else {
			st.TestConditions = testValues
		}
		if stTestCommand.Valid {
			st.TestCommand = stTestCommand.String
		}
		if stNotes.Valid {
			st.AgentNotes = stNotes.String
		}
		if stReviewed.Valid {
			st.ReviewedTimestamp = stReviewed.String
		}
		if stPass.Valid {
			st.PassTimestamp = stPass.String
		}
		if stFail.Valid {
			st.FailTimestamp = stFail.String
		}
		if stStatus.Valid {
			st.Status = stStatus.String
		}
		ticket.Subtasks = append(ticket.Subtasks, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := validateTicket(ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func SaveTicket(ticket *Ticket) error {
	if ticket == nil {
		return fmt.Errorf("ticket is nil")
	}
	if err := validateTicket(ticket); err != nil {
		return err
	}
	db, err := openTicketDB(ticket.ID)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM subtasks WHERE ticket_id = ?`, ticket.ID); err != nil {
		return err
	}

	tagPayload, err := encodeStringSlice(ticket.Tags)
	if err != nil {
		return err
	}
	state := strings.TrimSpace(ticket.State)
	if state == "" {
		state = "new"
	}
	if _, err := tx.Exec(`INSERT INTO tickets (id, name, tags, description, state, agent_summary, start_time, last_summary_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET 
			name = excluded.name, 
			tags = excluded.tags, 
			description = excluded.description,
			state = excluded.state,
			agent_summary = excluded.agent_summary,
			start_time = excluded.start_time,
			last_summary_time = excluded.last_summary_time`,
		ticket.ID, ticket.Name, tagPayload, ticket.Description,
		state, ticket.AgentSummary, ticket.StartTime, ticket.LastSummaryTime); err != nil {
		return err
	}

	for i, st := range ticket.Subtasks {
		stTagPayload, err := encodeStringSlice(st.Tags)
		if err != nil {
			return err
		}
		stDepsPayload, err := encodeStringSlice(st.Dependencies)
		if err != nil {
			return err
		}
		stTestsPayload, err := encodeTestConditions(st.TestConditions)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO subtasks (
			ticket_id, position, name, tags, dependencies, description, test_conditions, test_command, agent_notes, reviewed_timestamp, pass_timestamp, fail_timestamp, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ticket.ID,
			i,
			st.Name,
			stTagPayload,
			stDepsPayload,
			st.Description,
			stTestsPayload,
			nullOrValue(st.TestCommand),
			nullOrValue(st.AgentNotes),
			nullOrValue(st.ReviewedTimestamp),
			nullOrValue(st.PassTimestamp),
			nullOrValue(st.FailTimestamp),
			nullOrValue(st.Status),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func ListTickets() ([]string, error) {
	root := filepath.Join("src", "tickets")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		ids = append(ids, name)
	}
	sort.Strings(ids)
	return ids, nil
}

func AppendTicketSummary(ticketID, subtaskName, content string) error {
	if ticketID == "" {
		return fmt.Errorf("ticket ID is empty")
	}
	db, err := openTicketDB(ticketID)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO ticket_summaries (ticket_id, subtask_name, timestamp, content) VALUES (?, ?, ?, ?)`,
		ticketID,
		nullOrValue(subtaskName),
		time.Now().Format(time.RFC3339),
		content,
	)
	return err
}

func ListTicketSummaries(ticketID string) ([]SummaryEntry, error) {
	db, err := openTicketDB(ticketID)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT subtask_name, timestamp, content FROM ticket_summaries WHERE ticket_id = ? ORDER BY timestamp ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []SummaryEntry
	for rows.Next() {
		var e SummaryEntry
		var subtask sql.NullString
		if err := rows.Scan(&subtask, &e.Timestamp, &e.Content); err != nil {
			return nil, err
		}
		e.SubtaskName = subtask.String
		e.TicketID = ticketID
		entries = append(entries, e)
	}
	return entries, nil
}

func GetLastTicketSummary(ticketID string) (*SummaryEntry, error) {
	db, err := openTicketDB(ticketID)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var e SummaryEntry
	var subtask sql.NullString
	err = db.QueryRow(`SELECT subtask_name, timestamp, content FROM ticket_summaries WHERE ticket_id = ? ORDER BY timestamp DESC LIMIT 1`, ticketID).Scan(&subtask, &e.Timestamp, &e.Content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	e.SubtaskName = subtask.String
	e.TicketID = ticketID
	return &e, nil
}

func GetLastTicketSummaryForSubtask(ticketID, subtaskName string) (*SummaryEntry, error) {
	db, err := openTicketDB(ticketID)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var e SummaryEntry
	err = db.QueryRow(
		`SELECT subtask_name, timestamp, content
		 FROM ticket_summaries
		 WHERE ticket_id = ? AND subtask_name = ?
		 ORDER BY timestamp DESC
		 LIMIT 1`,
		ticketID, subtaskName,
	).Scan(&e.SubtaskName, &e.Timestamp, &e.Content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	e.TicketID = ticketID
	return &e, nil
}

func SearchTicketSummaries(query string) ([]SummaryEntry, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is empty")
	}
	ids, err := ListTickets()
	if err != nil {
		return nil, err
	}
	var out []SummaryEntry
	for _, id := range ids {
		db, err := openTicketDB(id)
		if err != nil {
			// Ticket folder may exist without DB yet; skip
			continue
		}
		rows, qerr := db.Query(`SELECT ticket_id, subtask_name, timestamp, content FROM ticket_summaries WHERE content LIKE ? OR subtask_name LIKE ? ORDER BY timestamp DESC`,
			"%"+query+"%", "%"+query+"%")
		if qerr != nil {
			db.Close()
			continue
		}
		for rows.Next() {
			var e SummaryEntry
			var subtask sql.NullString
			if err := rows.Scan(&e.TicketID, &subtask, &e.Timestamp, &e.Content); err != nil {
				continue
			}
			e.SubtaskName = subtask.String
			out = append(out, e)
		}
		rows.Close()
		db.Close()
	}
	// Most recent first across all tickets
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp > out[j].Timestamp })
	return out, nil
}

func AppendTicketLogEntry(ticketID, entryType, message, subtask string) error {
	if ticketID == "" {
		return fmt.Errorf("ticket ID is empty")
	}
	if entryType == "" {
		return fmt.Errorf("entry type is empty")
	}
	db, err := openTicketDB(ticketID)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO ticket_logs (ticket_id, timestamp, entry_type, message, subtask) VALUES (?, ?, ?, ?, ?)`,
		ticketID,
		time.Now().Format(time.RFC3339),
		entryType,
		message,
		nullOrValue(subtask),
	)
	return err
}

func GetLogEntries(ticketID string) ([]LogEntry, error) {
	if ticketID == "" {
		return nil, fmt.Errorf("ticket ID is empty")
	}
	db, err := openTicketDB(ticketID)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT timestamp, entry_type, message, subtask FROM ticket_logs WHERE ticket_id = ? ORDER BY timestamp ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		var subtask sql.NullString
		if err := rows.Scan(&e.Timestamp, &e.EntryType, &e.Message, &subtask); err != nil {
			return nil, err
		}
		e.Subtask = subtask.String
		entries = append(entries, e)
	}
	return entries, nil
}

func SetCurrentTicket(ticketID string) error {
	if ticketID == "" {
		return fmt.Errorf("ticket ID is empty")
	}
	if err := os.MkdirAll(filepath.Join("src", "tickets"), 0755); err != nil {
		return err
	}
	return os.WriteFile(currentTicketPath(), []byte(ticketID+"\n"), 0644)
}

func SetCurrentTicketMode(mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return fmt.Errorf("mode is empty")
	}
	if err := os.MkdirAll(filepath.Join("src", "tickets"), 0755); err != nil {
		return err
	}
	return os.WriteFile(currentTicketModePath(), []byte(mode+"\n"), 0644)
}

func GetCurrentTicketID() (string, error) {
	content, err := os.ReadFile(currentTicketPath())
	if err != nil {
		return "", fmt.Errorf("no current ticket set; run 'dialtone.sh ticket start <ticket-name>' or 'dialtone.sh ticket review <ticket-name>'")
	}
	id := strings.TrimSpace(string(content))
	if id == "" {
		return "", fmt.Errorf("no current ticket set; run 'dialtone.sh ticket start <ticket-name>' or 'dialtone.sh ticket review <ticket-name>'")
	}
	return id, nil
}

func GetCurrentTicketMode() string {
	// Default to start to preserve older behavior if mode file is missing.
	content, err := os.ReadFile(currentTicketModePath())
	if err != nil {
		return "start"
	}
	mode := strings.TrimSpace(string(content))
	if mode == "" {
		return "start"
	}
	return mode
}

func nullOrValue(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func encodeStringSlice(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeStringSlice(value sql.NullString) ([]string, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(value.String), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func encodeTestConditions(values []TestCondition) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeTestConditions(value sql.NullString) ([]TestCondition, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	var items []TestCondition
	if err := json.Unmarshal([]byte(value.String), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func validateTicket(ticket *Ticket) error {
	if ticket.ID == "" {
		return fmt.Errorf("ticket is missing '# Name:' header")
	}

	validStatuses := map[string]bool{
		"todo":     true,
		"progress": true,
		"done":     true,
		"failed":   true,
		"skipped":  true,
		"":         true,
	}

	for _, st := range ticket.Subtasks {
		if st.Name == "" {
			return fmt.Errorf("subtask is missing '- name:' field")
		}
		if !validStatuses[st.Status] {
			return fmt.Errorf("subtask %s has invalid status: %s", st.Name, st.Status)
		}

		if st.PassTimestamp != "" && st.FailTimestamp != "" {
			passTime, errP := time.Parse(time.RFC3339, st.PassTimestamp)
			failTime, errF := time.Parse(time.RFC3339, st.FailTimestamp)
			if errP == nil && errF == nil {
				if failTime.After(passTime) {
					return fmt.Errorf("[REGRESSION] subtask %s failed at %s, which is after it passed at %s", st.Name, st.FailTimestamp, st.PassTimestamp)
				}
			}
		}
	}

	return nil
}

func SaveKey(key *KeyEntry) error {
	db, err := openKeysDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`INSERT INTO keys (name, encrypted_value, salt, nonce) 
		VALUES (?, ?, ?, ?) 
		ON CONFLICT(name) DO UPDATE SET 
			encrypted_value = excluded.encrypted_value,
			salt = excluded.salt,
			nonce = excluded.nonce`,
		key.Name, key.EncryptedValue, key.Salt, key.Nonce)
	return err
}

func GetKey(name string) (*KeyEntry, error) {
	db, err := openKeysDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var k KeyEntry
	err = db.QueryRow(`SELECT name, encrypted_value, salt, nonce FROM keys WHERE name = ?`, name).Scan(&k.Name, &k.EncryptedValue, &k.Salt, &k.Nonce)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}

func ListKeyNames() ([]string, error) {
	db, err := openKeysDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name FROM keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func DeleteKey(name string) error {
	db, err := openKeysDB()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`DELETE FROM keys WHERE name = ?`, name)
	return err
}

func DeleteTicket(ticketID string) error {
	// Per-ticket DB: delete the DuckDB file if it exists.
	dbPath := ticketDBPathFor(ticketID)
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	// If this was the current ticket, clear the pointer.
	if cur, err := GetCurrentTicketID(); err == nil && cur == ticketID {
		_ = os.Remove(currentTicketPath())
	}
	return nil
}

// LoadBackupDB imports tickets from a backup duckdb file into the main database
func LoadBackupDB(backupPath string) error {
	// Open backup database
	backupDB, err := sql.Open("duckdb", backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup: %w", err)
	}
	defer backupDB.Close()

	// Import tickets into per-ticket DBs
	// Older backups may not have `state`, so attempt state-aware query first.
	hasState := true
	rows, err := backupDB.Query(`SELECT id, name, tags, description, state, agent_summary, start_time, last_summary_time FROM tickets`)
	if err != nil {
		// Fallback for older schemas without `state`.
		hasState = false
		rows, err = backupDB.Query(`SELECT id, name, tags, description, agent_summary, start_time, last_summary_time FROM tickets`)
		if err != nil {
			return fmt.Errorf("failed to query tickets: %w", err)
		}
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		var tags, description, state, agentSummary, startTime, lastSummaryTime sql.NullString
		if hasState {
			if err := rows.Scan(&id, &name, &tags, &description, &state, &agentSummary, &startTime, &lastSummaryTime); err != nil {
				return err
			}
		} else {
			if err := rows.Scan(&id, &name, &tags, &description, &agentSummary, &startTime, &lastSummaryTime); err != nil {
				return err
			}
		}
		mainDB, err := openTicketDB(id)
		if err != nil {
			return fmt.Errorf("failed to open ticket db for %s: %w", id, err)
		}
		_, err = mainDB.Exec(`INSERT INTO tickets (id, name, tags, description, state, agent_summary, start_time, last_summary_time)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET 
				name = excluded.name, 
				tags = excluded.tags, 
				description = excluded.description,
				state = excluded.state,
				agent_summary = excluded.agent_summary,
				start_time = excluded.start_time,
				last_summary_time = excluded.last_summary_time`,
			id, name, tags.String, description.String, state.String, agentSummary.String, startTime.String, lastSummaryTime.String)
		mainDB.Close()
		if err != nil {
			return fmt.Errorf("failed to insert ticket %s: %w", id, err)
		}
	}

	// Import subtasks
	subtaskRows, err := backupDB.Query(`SELECT ticket_id, position, name, tags, dependencies, description, test_conditions, test_command, agent_notes, pass_timestamp, fail_timestamp, status FROM subtasks`)
	if err != nil {
		// Table might not exist in older backups
		if !strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("failed to query subtasks: %w", err)
		}
	} else {
		defer subtaskRows.Close()
		for subtaskRows.Next() {
			var ticketID, name string
			var position int
			var tags, deps, description, testConds, testCommand, agentNotes, passTs, failTs, status sql.NullString
			if err := subtaskRows.Scan(&ticketID, &position, &name, &tags, &deps, &description, &testConds, &testCommand, &agentNotes, &passTs, &failTs, &status); err != nil {
				return err
			}
			mainDB, err := openTicketDB(ticketID)
			if err != nil {
				return fmt.Errorf("failed to open ticket db for %s: %w", ticketID, err)
			}
			// Delete existing subtasks for this ticket first to avoid duplicates
			mainDB.Exec(`DELETE FROM subtasks WHERE ticket_id = ? AND name = ?`, ticketID, name)
			_, err = mainDB.Exec(`INSERT INTO subtasks (ticket_id, position, name, tags, dependencies, description, test_conditions, test_command, agent_notes, pass_timestamp, fail_timestamp, status)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				ticketID, position, name, tags.String, deps.String, description.String, testConds.String, testCommand.String, agentNotes.String, passTs.String, failTs.String, status.String)
			mainDB.Close()
			if err != nil {
				return fmt.Errorf("failed to insert subtask %s/%s: %w", ticketID, name, err)
			}
		}
	}

	// Import summaries
	summaryRows, err := backupDB.Query(`SELECT ticket_id, subtask_name, timestamp, content FROM ticket_summaries`)
	if err != nil {
		if !strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("failed to query summaries: %w", err)
		}
	} else {
		defer summaryRows.Close()
		for summaryRows.Next() {
			var ticketID, timestamp, content string
			var subtaskName sql.NullString
			if err := summaryRows.Scan(&ticketID, &subtaskName, &timestamp, &content); err != nil {
				return err
			}
			mainDB, err := openTicketDB(ticketID)
			if err != nil {
				return fmt.Errorf("failed to open ticket db for %s: %w", ticketID, err)
			}
			// Check if already exists
			var count int
			mainDB.QueryRow(`SELECT COUNT(*) FROM ticket_summaries WHERE ticket_id = ? AND timestamp = ?`, ticketID, timestamp).Scan(&count)
			if count == 0 {
				mainDB.Exec(`INSERT INTO ticket_summaries (ticket_id, subtask_name, timestamp, content) VALUES (?, ?, ?, ?)`,
					ticketID, subtaskName.String, timestamp, content)
			}
			mainDB.Close()
		}
	}

	// Import logs
	logRows, err := backupDB.Query(`SELECT ticket_id, timestamp, entry_type, message, subtask FROM ticket_logs`)
	if err != nil {
		if !strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("failed to query logs: %w", err)
		}
	} else {
		defer logRows.Close()
		for logRows.Next() {
			var ticketID, timestamp, entryType, message string
			var subtask sql.NullString
			if err := logRows.Scan(&ticketID, &timestamp, &entryType, &message, &subtask); err != nil {
				return err
			}
			mainDB, err := openTicketDB(ticketID)
			if err != nil {
				return fmt.Errorf("failed to open ticket db for %s: %w", ticketID, err)
			}
			// Check if already exists
			var count int
			mainDB.QueryRow(`SELECT COUNT(*) FROM ticket_logs WHERE ticket_id = ? AND timestamp = ?`, ticketID, timestamp).Scan(&count)
			if count == 0 {
				mainDB.Exec(`INSERT INTO ticket_logs (ticket_id, timestamp, entry_type, message, subtask) VALUES (?, ?, ?, ?, ?)`,
					ticketID, timestamp, entryType, message, subtask.String)
			}
			mainDB.Close()
		}
	}

	return nil
}
