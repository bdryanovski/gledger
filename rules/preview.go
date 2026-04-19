package rules

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"doublebook/utils"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// ---------------------------------------------------------------------------
// Preview - Analyze files for interactive mapping
// ---------------------------------------------------------------------------

// FilePreview contains analyzed data from a file for interactive mapping.
type FilePreview struct {
	FilePath   string      `json:"file_path"`
	FileType   string      `json:"file_type"` // "csv" or "excel"
	TotalRows  int         `json:"total_rows"`
	Columns    []ColumnDef `json:"columns"`
	SampleRows [][]string  `json:"sample_rows"` // first N rows of actual data
	Sheets     []string    `json:"sheets"`      // for Excel files
}

// PreviewOptions configures how to preview a file.
type PreviewOptions struct {
	Delimiter    string // CSV delimiter (default: auto-detect)
	Encoding     string // file encoding (default: utf-8)
	SkipLines    int    // header lines to skip (default: 0 for preview)
	SampleCount  int    // number of sample rows to include (default: 5)
	SheetName    string // for Excel: which sheet to use
	SheetIndex   int    // for Excel: sheet index (0-based)
}

// DefaultPreviewOptions returns sensible defaults for previewing.
func DefaultPreviewOptions() PreviewOptions {
	return PreviewOptions{
		Delimiter:   "",  // auto-detect
		Encoding:    "utf-8",
		SkipLines:   0,
		SampleCount: 5,
	}
}

// PreviewFile analyzes a CSV or Excel file and returns column info with samples.
func PreviewFile(path string, opts PreviewOptions) (*FilePreview, error) {
	path = utils.ExpandHome(path)
	
	if opts.SampleCount <= 0 {
		opts.SampleCount = 5
	}
	
	ext := strings.ToLower(filepath.Ext(path))
	
	switch ext {
	case ".csv", ".tsv", ".txt":
		return previewCSV(path, opts)
	case ".xlsx", ".xls":
		return previewExcel(path, opts)
	default:
		// Try CSV first
		preview, err := previewCSV(path, opts)
		if err == nil && len(preview.Columns) > 1 {
			return preview, nil
		}
		return nil, fmt.Errorf("unsupported file type %q", ext)
	}
}

// ---------------------------------------------------------------------------
// CSV Preview
// ---------------------------------------------------------------------------

func previewCSV(path string, opts PreviewOptions) (*FilePreview, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()
	
	// Apply encoding
	var reader io.Reader = f
	if opts.Encoding != "" && opts.Encoding != "utf-8" {
		reader, err = decodingReader(f, opts.Encoding)
		if err != nil {
			return nil, fmt.Errorf("applying encoding: %w", err)
		}
	}
	
	// Auto-detect delimiter if not specified
	delimiter := opts.Delimiter
	if delimiter == "" {
		delimiter = detectDelimiter(path)
	}
	
	csvReader := csv.NewReader(reader)
	csvReader.Comma = rune(delimiter[0])
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1
	
	// Read all rows (limited)
	var allRows [][]string
	maxRows := opts.SampleCount + opts.SkipLines + 1 + 100 // headers + samples + buffer
	
	for i := 0; i < maxRows; i++ {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip bad rows for preview
		}
		allRows = append(allRows, row)
	}
	
	if len(allRows) == 0 {
		return nil, fmt.Errorf("no data rows found")
	}
	
	// Count total rows
	totalRows := len(allRows)
	
	// Re-read for accurate count if we hit the limit
	if totalRows == maxRows {
		f.Seek(0, 0)
		reader = f
		if opts.Encoding != "" && opts.Encoding != "utf-8" {
			reader, _ = decodingReader(f, opts.Encoding)
		}
		csvReader = csv.NewReader(reader)
		csvReader.Comma = rune(delimiter[0])
		csvReader.LazyQuotes = true
		csvReader.FieldsPerRecord = -1
		
		totalRows = 0
		for {
			_, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err == nil {
				totalRows++
			}
		}
	}
	
	// First row is typically headers
	headers := allRows[0]
	dataRows := allRows[1:]
	
	// Build column definitions
	columns := make([]ColumnDef, len(headers))
	for i, header := range headers {
		columns[i] = ColumnDef{
			Index:   i,
			Name:    strings.TrimSpace(header),
			Samples: make([]string, 0, opts.SampleCount),
		}
	}
	
	// Collect samples
	sampleRows := make([][]string, 0, opts.SampleCount)
	for i, row := range dataRows {
		if i >= opts.SampleCount {
			break
		}
		sampleRows = append(sampleRows, row)
		
		for j := range columns {
			if j < len(row) {
				sample := strings.TrimSpace(row[j])
				if sample != "" && len(columns[j].Samples) < opts.SampleCount {
					// Avoid duplicates
					isDup := false
					for _, existing := range columns[j].Samples {
						if existing == sample {
							isDup = true
							break
						}
					}
					if !isDup {
						columns[j].Samples = append(columns[j].Samples, sample)
					}
				}
			}
		}
	}
	
	return &FilePreview{
		FilePath:   path,
		FileType:   "csv",
		TotalRows:  totalRows - 1, // exclude header
		Columns:    columns,
		SampleRows: sampleRows,
	}, nil
}

// detectDelimiter tries to auto-detect the CSV delimiter.
func detectDelimiter(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ","
	}
	defer f.Close()
	
	// Read first few KB
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	content := string(buf[:n])
	
	// Count potential delimiters in first line
	firstLine := strings.Split(content, "\n")[0]
	
	delimiters := []struct {
		char  string
		count int
	}{
		{",", strings.Count(firstLine, ",")},
		{";", strings.Count(firstLine, ";")},
		{"\t", strings.Count(firstLine, "\t")},
		{"|", strings.Count(firstLine, "|")},
	}
	
	best := ","
	bestCount := 0
	for _, d := range delimiters {
		if d.count > bestCount {
			best = d.char
			bestCount = d.count
		}
	}
	
	return best
}

// ---------------------------------------------------------------------------
// Excel Preview
// ---------------------------------------------------------------------------

func previewExcel(path string, opts PreviewOptions) (*FilePreview, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("opening Excel file: %w", err)
	}
	defer f.Close()
	
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}
	
	// Select sheet
	sheetName := sheets[0]
	if opts.SheetName != "" {
		sheetName = opts.SheetName
	} else if opts.SheetIndex > 0 && opts.SheetIndex < len(sheets) {
		sheetName = sheets[opts.SheetIndex]
	}
	
	// Read rows
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("reading sheet %q: %w", sheetName, err)
	}
	
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data rows found in sheet %q", sheetName)
	}
	
	// First row is headers
	headers := rows[0]
	dataRows := rows[1:]
	
	// Build column definitions
	columns := make([]ColumnDef, len(headers))
	for i, header := range headers {
		columns[i] = ColumnDef{
			Index:   i,
			Name:    strings.TrimSpace(header),
			Samples: make([]string, 0, opts.SampleCount),
		}
	}
	
	// Collect samples
	sampleRows := make([][]string, 0, opts.SampleCount)
	for i, row := range dataRows {
		if i >= opts.SampleCount {
			break
		}
		sampleRows = append(sampleRows, row)
		
		for j := range columns {
			if j < len(row) {
				sample := strings.TrimSpace(row[j])
				if sample != "" && len(columns[j].Samples) < opts.SampleCount {
					isDup := false
					for _, existing := range columns[j].Samples {
						if existing == sample {
							isDup = true
							break
						}
					}
					if !isDup {
						columns[j].Samples = append(columns[j].Samples, sample)
					}
				}
			}
		}
	}
	
	return &FilePreview{
		FilePath:   path,
		FileType:   "excel",
		TotalRows:  len(dataRows),
		Columns:    columns,
		SampleRows: sampleRows,
		Sheets:     sheets,
	}, nil
}

// ---------------------------------------------------------------------------
// Encoding helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Suggestion helpers
// ---------------------------------------------------------------------------

// SuggestFieldMapping analyzes a column and suggests what field it might map to.
func SuggestFieldMapping(col ColumnDef) string {
	name := strings.ToLower(col.Name)
	
	// Date patterns
	datePatterns := []string{"date", "дата", "datum", "fecha", "data"}
	for _, p := range datePatterns {
		if strings.Contains(name, p) {
			return "date"
		}
	}
	
	// Amount patterns
	if strings.Contains(name, "amount") || strings.Contains(name, "сума") || 
	   strings.Contains(name, "sum") || strings.Contains(name, "value") ||
	   strings.Contains(name, "total") {
		return "amount"
	}
	
	// Debit patterns
	if strings.Contains(name, "debit") || strings.Contains(name, "дебит") ||
	   strings.Contains(name, "withdrawal") || strings.Contains(name, "expense") {
		return "debit_amount"
	}
	
	// Credit patterns
	if strings.Contains(name, "credit") || strings.Contains(name, "кредит") ||
	   strings.Contains(name, "deposit") || strings.Contains(name, "income") {
		return "credit_amount"
	}
	
	// Description patterns
	if strings.Contains(name, "description") || strings.Contains(name, "desc") ||
	   strings.Contains(name, "narration") || strings.Contains(name, "memo") ||
	   strings.Contains(name, "details") || strings.Contains(name, "note") ||
	   strings.Contains(name, "описание") || strings.Contains(name, "merchant") {
		return "description"
	}
	
	// Reference patterns
	if strings.Contains(name, "reference") || strings.Contains(name, "ref") ||
	   strings.Contains(name, "id") || strings.Contains(name, "transaction") {
		return "reference"
	}
	
	// Currency patterns
	if strings.Contains(name, "currency") || strings.Contains(name, "валута") ||
	   strings.Contains(name, "ccy") {
		return "currency"
	}
	
	// Category patterns
	if strings.Contains(name, "category") || strings.Contains(name, "категория") ||
	   strings.Contains(name, "type") {
		return "tag:category"
	}
	
	// Balance - usually informational, skip
	if strings.Contains(name, "balance") || strings.Contains(name, "салдо") {
		return ""
	}
	
	return ""
}

// DetectDateFormat tries to detect the date format from sample values.
func DetectDateFormat(samples []string) string {
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		"02.01.2006",
		"2006.01.02",
		"Jan 2, 2006",
		"January 2, 2006",
		"2 Jan 2006",
		"02 Jan 2006",
	}
	
	for _, sample := range samples {
		sample = strings.TrimSpace(sample)
		if sample == "" {
			continue
		}
		
		for _, format := range formats {
			if _, err := parseDate(sample, format); err == nil {
				return format
			}
		}
	}
	
	return "2006-01-02" // default
}

func parseDate(s, format string) (string, error) {
	_, err := time.Parse(format, s)
	if err != nil {
		return "", err
	}
	return s, nil
}
