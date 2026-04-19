// Package handlers contains HTTP handler functions for the DoubleBook REST API.
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"doublebook/core/currency"
	"doublebook/infra/db"
	"doublebook/engine/fql"
	"doublebook/ingest/legacy"
	"doublebook/engine/interpreter"
	"doublebook/core/journal"
	"doublebook/infra/utils"
)

// ---------------------------------------------------------------------------
// Handler collection
// ---------------------------------------------------------------------------

// Handlers holds shared dependencies for all API handlers.
type Handlers struct {
	interp    *interpreter.Interpreter
	db        *db.DB
	converter *currency.CachingConverter
}

// New creates a Handlers instance.
func New(
	interp *interpreter.Interpreter,
	database *db.DB,
	conv *currency.CachingConverter,
) *Handlers {
	return &Handlers{interp: interp, db: database, converter: conv}
}

// ---------------------------------------------------------------------------
// GET /api/transactions
// ---------------------------------------------------------------------------

func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	q := r.URL.Query()
	filter := interpreter.Filter{
		BeginDate:   q.Get("begin"),
		EndDate:     q.Get("end"),
		Account:     q.Get("account"),
		Description: q.Get("desc"),
	}

	txns := h.interp.FilteredTransactions(filter)

	// Pagination.
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	total := len(txns)
	if limit > 0 {
		if offset >= len(txns) {
			txns = nil
		} else {
			txns = txns[offset:]
			if limit < len(txns) {
				txns = txns[:limit]
			}
		}
	}

	type postingJSON struct {
		Account  string  `json:"account"`
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	type txnJSON struct {
		ID          string            `json:"id"`
		Date        string            `json:"date"`
		Description string            `json:"description"`
		Status      string            `json:"status"`
		Postings    []postingJSON     `json:"postings"`
		Tags        map[string]string `json:"tags"`
	}

	out := make([]txnJSON, 0, len(txns))
	for _, t := range txns {
		tj := txnJSON{
			ID:          t.ID,
			Date:        t.Date.Format("2006-01-02"),
			Description: t.Description,
			Status:      t.Status,
			Tags:        t.Tags,
			Postings:    make([]postingJSON, len(t.Postings)),
		}
		for i, p := range t.Postings {
			tj.Postings[i] = postingJSON{
				Account:  p.Account,
				Amount:   p.Amount.Value,
				Currency: p.Amount.Currency,
			}
		}
		out = append(out, tj)
	}

	respond(w, http.StatusOK, map[string]interface{}{
		"transactions": out,
		"total":        total,
	})
}

// ---------------------------------------------------------------------------
// GET /api/accounts
// ---------------------------------------------------------------------------

func (h *Handlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	nodes := h.interp.CalculateBalancesTree(interpreter.Filter{})
	groups := interpreter.GroupAccountsByType(nodes)

	typeFilter := strings.ToLower(r.URL.Query().Get("type"))

	type accountJSON struct {
		Name     string  `json:"name"`
		Type     string  `json:"type"`
		Amount   float64 `json:"balance"`
		Currency string  `json:"currency"`
	}

	var out []accountJSON
	order := []string{"assets", "liabilities", "equity", "income", "expenses", "other"}
	for _, t := range order {
		if typeFilter != "" && t != typeFilter {
			continue
		}
		for _, node := range groups[t] {
			out = append(out, accountJSON{
				Name:     node.FullName,
				Type:     t,
				Amount:   node.Amount.Value,
				Currency: node.Amount.Currency,
			})
		}
	}

	respond(w, http.StatusOK, map[string]interface{}{"accounts": out})
}

// ---------------------------------------------------------------------------
// GET /api/reports/balance
// ---------------------------------------------------------------------------

func (h *Handlers) BalanceReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	filter := interpreter.Filter{BeginDate: q.Get("begin"), EndDate: q.Get("end")}
	nodes := h.interp.CalculateBalancesTree(filter)
	groups := interpreter.GroupAccountsByType(nodes)

	type acctEntry struct {
		Account  string  `json:"account"`
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	}
	result := make(map[string][]acctEntry)
	order := []string{"assets", "liabilities", "equity", "income", "expenses", "other"}
	for _, t := range order {
		for _, n := range groups[t] {
			result[t] = append(result[t], acctEntry{
				Account:  n.FullName,
				Amount:   n.Amount.Value,
				Currency: n.Amount.Currency,
			})
		}
	}
	respond(w, http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// GET /api/reports/income-statement
// ---------------------------------------------------------------------------

func (h *Handlers) IncomeStatement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	filter := interpreter.Filter{BeginDate: q.Get("begin"), EndDate: q.Get("end")}
	stmt := h.interp.GenerateIncomeStatement(filter)

	type amtEntry struct {
		Value    float64 `json:"value"`
		Currency string  `json:"currency"`
	}
	revMap := make(map[string]amtEntry)
	for acc, a := range stmt.Revenues {
		revMap[acc] = amtEntry{Value: a.Value, Currency: a.Currency}
	}
	expMap := make(map[string]amtEntry)
	for acc, a := range stmt.Expenses {
		expMap[acc] = amtEntry{Value: a.Value, Currency: a.Currency}
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"revenues":   revMap,
		"expenses":   expMap,
		"net_income": amtEntry{Value: stmt.NetIncome.Value, Currency: stmt.NetIncome.Currency},
	})
}

// ---------------------------------------------------------------------------
// POST /api/fql
// ---------------------------------------------------------------------------

func (h *Handlers) FQLQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Query == "" {
		respondError(w, http.StatusBadRequest, "query field is required")
		return
	}

	columns, rows, err := fql.RunQuery(body.Query, h.db)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Convert [][]interface{} to [][]interface{} (already correct type).
	respond(w, http.StatusOK, map[string]interface{}{
		"columns":   columns,
		"rows":      rows,
		"row_count": len(rows),
	})
}

// ---------------------------------------------------------------------------
// GET /api/exchange-rates
// ---------------------------------------------------------------------------

func (h *Handlers) ExchangeRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	from := q.Get("from")
	to := q.Get("to")
	date := q.Get("date")
	if from == "" || to == "" || date == "" {
		respondError(w, http.StatusBadRequest, "from, to, and date query params are required")
		return
	}
	if h.converter == nil {
		respondError(w, http.StatusServiceUnavailable, "currency converter not configured")
		return
	}
	rate, err := h.converter.GetRate(date, from, to)
	if err != nil {
		respondError(w, http.StatusBadGateway, fmt.Sprintf("rate lookup failed: %v", err))
		return
	}
	respond(w, http.StatusOK, map[string]interface{}{
		"from": strings.ToUpper(from),
		"to":   strings.ToUpper(to),
		"date": date,
		"rate": rate,
	})
}

// ---------------------------------------------------------------------------
// POST /api/import
// ---------------------------------------------------------------------------

func (h *Handlers) ImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "multipart form parse error: "+err.Error())
		return
	}

	// Get importmap file.
	mapFile, _, err := r.FormFile("map")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing 'map' file in form")
		return
	}
	defer mapFile.Close()

	// Get CSV file.
	csvFile, csvHeader, err := r.FormFile("csv")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing 'csv' file in form")
		return
	}
	defer csvFile.Close()

	dryRun := r.FormValue("dry_run") == "true"

	// Write importmap to temp file for LoadImportMap.
	tmpMap := writeTempToPath(mapFile, "*.importmap.json")
	defer os.Remove(tmpMap)

	// Write CSV to temp file.
	_ = csvHeader
	tmpCSV := writeTempToPath(csvFile, "*.csv")
	defer os.Remove(tmpCSV)

	imap, err := legacy.LoadImportMap(tmpMap)
	if err != nil {
		respondError(w, http.StatusBadRequest, "importmap error: "+err.Error())
		return
	}

	existingIDs := legacy.ExtractIDs(h.interp.GetTransactions())
	result, err := legacy.ImportCSV(tmpCSV, imap, existingIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "import error: "+err.Error())
		return
	}

	var writeErrors []string
	if !dryRun && len(result.Imported) > 0 {
		cfg := h.interp.GetConfig()
		dataDir := utils.ExpandHome(cfg.DataDir)
		for i, txn := range result.Imported {
			if err := journal.AppendTransaction(cfg.JournalName, dataDir, txn); err != nil {
				writeErrors = append(writeErrors, fmt.Sprintf("write error for txn %d: %v", i+1, err))
			}
		}
	}

	errStrs := make([]string, 0, len(result.Errors)+len(writeErrors))
	for _, e := range result.Errors {
		errStrs = append(errStrs, fmt.Sprintf("row %d: %s", e.Row, e.Reason))
	}
	errStrs = append(errStrs, writeErrors...)
	respond(w, http.StatusOK, map[string]interface{}{
		"imported": len(result.Imported),
		"skipped":  result.Skipped,
		"errors":   errStrs,
		"total":    result.TotalRows,
		"dry_run":  dryRun,
	})
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

func respond(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	// Error writing to HTTP response cannot be meaningfully handled;
	// client may have disconnected. Silently ignore.
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}

// ---------------------------------------------------------------------------
// I/O helpers
// ---------------------------------------------------------------------------

// writeTempToPath copies src into a new temp file matching pattern and
// returns the file path. Returns "" on error.
func writeTempToPath(src io.Reader, pattern string) string {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := io.Copy(f, src); err != nil {
		return ""
	}
	return f.Name()
}
