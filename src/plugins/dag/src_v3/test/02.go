package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type dagTableAPIResponse struct {
	Rows []struct {
		Key    string `json:"key"`
		Value  string `json:"value"`
		Status string `json:"status"`
	} `json:"rows"`
}

func fetchDagTableRowsFromAPI(baseURL string) (*dagTableAPIResponse, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get(baseURL + "/api/dag-table")
	if err != nil {
		return nil, fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api status not OK: %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		return nil, fmt.Errorf("api content-type is not application/json: %q", contentType)
	}
	var out dagTableAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode api json: %w", err)
	}
	if len(out.Rows) < 6 {
		return nil, fmt.Errorf("api returned too few rows: %d", len(out.Rows))
	}
	required := map[string]bool{
		"node_count":                      false,
		"edge_count":                      false,
		"layer_count":                     false,
		"graph_edge_match_count":          false,
		"shortest_path_hops_root_to_leaf": false,
		"rank_violation_count":            false,
	}
	for _, row := range out.Rows {
		if row.Status != "OK" {
			return nil, fmt.Errorf("api row %q has non-OK status %q", row.Key, row.Status)
		}
		if _, ok := required[row.Key]; ok {
			required[row.Key] = true
		}
	}
	for key, present := range required {
		if !present {
			return nil, fmt.Errorf("api missing required metric row %q", key)
		}
	}
	return &out, nil
}

func Run02DagTableSectionValidation(ctx *testCtx) (string, error) {
	browser, err := ctx.browser()
	if err != nil {
		return "", err
	}
	if err := ctx.waitHTTPReady(ctx.appURL("/"), 12*time.Second); err != nil {
		return "", fmt.Errorf("dev server not ready before table validation: %w", err)
	}

	apiRows, err := fetchDagTableRowsFromAPI(ctx.appURL(""))
	if err != nil {
		return "", err
	}

	var tableOK bool
	var rowCount int
	ctx.appendThought("table validation: load dag-table and wait for ready")
	if err := ctx.navigate(ctx.appURL("/#dag-table")); err != nil {
		return "", err
	}
	if err := ctx.waitAria("DAG Table", "need table element for validation"); err != nil {
		return "", err
	}
	if err := ctx.waitAriaAttrEquals("DAG Table", "data-ready", "true", "wait for table ready flag", 8*time.Second); err != nil {
		return "", err
	}
	if err := browser.Run(chromedp.Tasks{
		chromedp.Evaluate(`
			(() => {
				const table = document.querySelector("table[aria-label='DAG Table']");
				if (!table) return false;
				const rows = table.querySelectorAll('tbody tr');
				if (rows.length < 6) return false;
				const first = rows[0].querySelector('td');
				if (!first) return false;
				if (first.textContent?.trim() !== 'node_count') return false;
				const bad = Array.from(rows).some((row) => {
					const cells = row.querySelectorAll('td');
					const key = cells[0]?.textContent?.trim() || '';
					const status = cells[2]?.textContent?.trim() || '';
					if (key === 'query_error' || key === 'dev_hint') return true;
					return status !== 'OK';
				});
				return !bad;
			})()
		`, &tableOK),
		chromedp.Evaluate(`(() => document.querySelectorAll("table[aria-label='DAG Table'] tbody tr").length)()`, &rowCount),
	}); err != nil {
		return "", err
	}
	if !tableOK {
		return "", fmt.Errorf("dag-table assertions failed")
	}
	if rowCount != len(apiRows.Rows) {
		return "", fmt.Errorf("table row count (%d) does not match api row count (%d)", rowCount, len(apiRows.Rows))
	}
	if err := ctx.captureShot("test_step_1_pre.png"); err != nil {
		return "", fmt.Errorf("capture table screenshot pre: %w", err)
	}
	if err := ctx.captureShot("test_step_1.png"); err != nil {
		return "", fmt.Errorf("capture table screenshot post: %w", err)
	}
	return "Loaded the DAG table, waited for `data-ready=true`, validated API parity and row status content, then captured pre/post table screenshots.", nil
}
