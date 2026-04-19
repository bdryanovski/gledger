package legacy

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"doublebook/core/ast"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// ---------------------------------------------------------------------------
// Result types
// ---------------------------------------------------------------------------

// ImportResult summarises a completed CSV import operation.
type ImportResult struct {
	Imported  []*ast.Transaction // new transactions to append to the journal
	Skipped   int                // rows already present (by ID) — not re-imported
	Errors    []ImportError      // non-fatal row-level parse problems
	TotalRows int                // total data rows examined (header lines excluded)
}

// ImportError describes a single row that could not be parsed.
type ImportError struct {
	Row     int    // 1-based row number after skipped header lines
	Content string // raw row content (joined with the delimiter)
	Reason  string // human-readable description of the failure
}

// ---------------------------------------------------------------------------
// ImportCSV — main entry point
// ---------------------------------------------------------------------------

// ImportCSV reads the CSV at csvPath, applies imap's column mappings, skips
// rows whose generated IDs are already in existingIDs, and returns the result.
//
// Non-fatal row errors are collected in result.Errors rather than aborting
// the whole import.
func ImportCSV(csvPath string, imap *ImportMap, existingIDs map[string]bool) (*ImportResult, error) {
	if existingIDs == nil {
		existingIDs = make(map[string]bool)
	}

	// Open + optionally transcode the file.
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", csvPath, err)
	}
	defer f.Close()

	rdr, err := decodingReader(f, imap.Encoding)
	if err != nil {
		return nil, err
	}

	// Configure csv.Reader.
	delims := []rune(imap.Delimiter)
	if len(delims) == 0 {
		delims = []rune{','}
	}
	cr := csv.NewReader(rdr)
	cr.Comma = delims[0]
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1

	// Skip header lines.
	for i := 0; i < imap.SkipLines; i++ {
		if _, err := cr.Read(); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("skipping header line %d: %w", i+1, err)
		}
	}

	result := &ImportResult{}
	seenInFile := make(map[string]bool)
	rowNum := 0
	delimStr := string(delims[0])

	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		result.TotalRows++

		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:    rowNum,
				Reason: "csv parse error: " + err.Error(),
			})
			continue
		}

		txn, skip, rowErr := processRow(row, rowNum, imap, existingIDs, seenInFile)
		if rowErr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:     rowNum,
				Content: strings.Join(row, delimStr),
				Reason:  rowErr.Error(),
			})
			continue
		}
		if skip {
			result.Skipped++
			continue
		}
		if txn == nil {
			continue // silently skipped (empty amount row)
		}

		seenInFile[txn.ID] = true
		result.Imported = append(result.Imported, txn)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Row processing
// ---------------------------------------------------------------------------

// processRow parses one CSV row into a Transaction.
//
// Returns:
//   - (txn, false, nil)  — new transaction to import
//   - (nil, true, nil)   — duplicate, should be counted as Skipped
//   - (nil, false, nil)  — silently skip (e.g. empty amount row)
//   - (nil, false, err)  — non-fatal parse error
func processRow(
	row []string,
	rowNum int,
	imap *ImportMap,
	existingIDs map[string]bool,
	seenInFile map[string]bool,
) (*ast.Transaction, bool, error) {

	col := func(idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	// Date.
	rawDate := col(imap.Columns.Date)
	if rawDate == "" {
		return nil, false, fmt.Errorf("empty date field")
	}
	date, err := time.Parse(imap.DateFormat, rawDate)
	if err != nil {
		return nil, false, fmt.Errorf("cannot parse date %q with format %q: %w",
			rawDate, imap.DateFormat, err)
	}

	// Reference (bank's own transaction ID) — extracted first so it can be
	// used as a description fallback.
	reference := col(ColIdx(imap.Columns.Reference))

	// Description — fall back to reference or a generic label so we never
	// write a transaction with an empty description (would break the parser).
	description := strings.TrimSpace(col(ColIdx(imap.Columns.Description)))
	if description == "" {
		if ref := strings.TrimSpace(reference); ref != "" {
			description = ref
		} else {
			// Use the ISO-formatted date (no slashes) to avoid lexer confusion.
			description = imap.Name + " " + date.Format("2006-01-02")
		}
	}

	// Amount.
	amountValue, isCredit, err := extractAmount(row, imap)
	if err != nil {
		return nil, false, err
	}
	if math.IsNaN(amountValue) {
		// Both amount columns empty — skip row silently.
		return nil, false, nil
	}

	// Unique ID for deduplication.
	amtStr := fmt.Sprintf("%.4f", amountValue)
	id := generateRowID(rawDate, amtStr, reference, description)

	// Deduplication checks.
	if existingIDs[id] || seenInFile[id] {
		return nil, true, nil
	}

	// Apply transforms.
	debitAcct := imap.DefaultDebitAccount
	creditAcct := imap.DefaultCreditAccount
	tags := map[string]string{"source": imap.Name}

	for _, tr := range imap.Transforms {
		if matchesTransform(tr, description, amountValue) {
			if tr.DebitAccount != "" {
				debitAcct = tr.DebitAccount
			}
			if tr.CreditAccount != "" {
				creditAcct = tr.CreditAccount
			}
			for k, v := range tr.Tags {
				tags[k] = v
			}
			if tr.Category != "" {
				tags["category"] = tr.Category
			}
			break // first matching transform wins
		}
	}

	// Currency (use per-row override if available).
	currency := imap.Currency
	if cidx := ColIdx(imap.Columns.Currency); cidx >= 0 {
		if c := col(cidx); c != "" {
			currency = c
		}
	}

	// Build the balanced transaction.
	//   debit  (money out): source −amount, debitAcct  +amount
	//   credit (money in):  source +amount, creditAcct −amount
	txn := ast.NewTransaction(date, description)
	txn.ID = id
	txn.Tags = tags

	if isCredit {
		txn.Postings = append(txn.Postings,
			ast.NewPosting(imap.SourceAccount, ast.Amount{Value: amountValue, Currency: currency}),
			ast.NewPosting(creditAcct, ast.Amount{Value: -amountValue, Currency: currency}),
		)
	} else {
		txn.Postings = append(txn.Postings,
			ast.NewPosting(imap.SourceAccount, ast.Amount{Value: -amountValue, Currency: currency}),
			ast.NewPosting(debitAcct, ast.Amount{Value: amountValue, Currency: currency}),
		)
	}

	return txn, false, nil
}

// ---------------------------------------------------------------------------
// Amount extraction
// ---------------------------------------------------------------------------

// extractAmount reads the amount value from the CSV row according to the
// importmap's column configuration.
//
// Returns (value, isCredit, nil) where value is always ≥ 0.
// Returns (NaN, false, nil) to signal "skip this row silently".
func extractAmount(row []string, imap *ImportMap) (float64, bool, error) {
	col := func(idx int) string {
		if idx < 0 || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	if imap.Columns.HasAmount() {
		raw := col(ColIdx(imap.Columns.Amount))
		if raw == "" {
			return math.NaN(), false, nil
		}
		v, err := parseNumber(raw)
		if err != nil {
			return 0, false, fmt.Errorf("cannot parse amount %q: %w", raw, err)
		}
		return math.Abs(v), v >= 0, nil
	}

	// Separate debit / credit columns.
	debitRaw := col(ColIdx(imap.Columns.DebitAmount))
	creditRaw := col(ColIdx(imap.Columns.CreditAmount))

	if debitRaw == "" && creditRaw == "" {
		return math.NaN(), false, nil
	}

	if debitRaw != "" {
		v, err := parseNumber(debitRaw)
		if err != nil {
			return 0, false, fmt.Errorf("cannot parse debit amount %q: %w", debitRaw, err)
		}
		return math.Abs(v), false, nil
	}

	v, err := parseNumber(creditRaw)
	if err != nil {
		return 0, false, fmt.Errorf("cannot parse credit amount %q: %w", creditRaw, err)
	}
	return math.Abs(v), true, nil
}

// parseNumber strips thousands separators and parses a float.
func parseNumber(s string) (float64, error) {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}

// ---------------------------------------------------------------------------
// Transform matching
// ---------------------------------------------------------------------------

func matchesTransform(tr Transform, description string, amount float64) bool {
	if tr.DescriptionContains != "" &&
		!strings.Contains(strings.ToLower(description), strings.ToLower(tr.DescriptionContains)) {
		return false
	}
	if tr.AmountMin != nil && amount < *tr.AmountMin {
		return false
	}
	if tr.AmountMax != nil && amount > *tr.AmountMax {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// ID generation
// ---------------------------------------------------------------------------

// generateRowID builds a stable 16-char hex ID for a CSV row.
// The bank's own reference is used when available (most reliable);
// otherwise a hash of date + amount + description is used.
func generateRowID(date, amount, reference, description string) string {
	key := strings.TrimSpace(reference)
	if key == "" {
		key = date + "|" + amount + "|" + strings.TrimSpace(description)
	}
	h := sha256.New()
	h.Write([]byte(key))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// ---------------------------------------------------------------------------
// Encoding
// ---------------------------------------------------------------------------

// decodingReader wraps r with a character-set decoder appropriate for enc.
// Returns r unchanged for UTF-8.
func decodingReader(r io.Reader, enc string) (io.Reader, error) {
	norm := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(enc, "-", ""), "_", ""))
	switch norm {
	case "utf8", "":
		return r, nil
	case "windows1251", "cp1251", "win1251":
		return transform.NewReader(r, charmap.Windows1251.NewDecoder()), nil
	case "windows1252", "cp1252":
		return transform.NewReader(r, charmap.Windows1252.NewDecoder()), nil
	case "iso88591", "latin1":
		return transform.NewReader(r, charmap.ISO8859_1.NewDecoder()), nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", enc)
	}
}
