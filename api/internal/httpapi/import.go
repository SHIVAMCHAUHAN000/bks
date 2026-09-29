package httpapi

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"automationtool/api/internal/lead"
)

type importRequest struct {
	CSV string `json:"csv"`
	URL string `json:"url"`
	// Mapping optionally overrides the detected columns: field -> column heading ("" = skip field).
	Mapping map[string]string `json:"mapping"`
	// Segment is applied to rows that have no segment column value.
	Segment string `json:"segment"`
}

type importTable struct {
	headers []string
	rows    [][]string
}

func (s *Server) loadImportTable(input importRequest) (importTable, int, error) {
	raw, source := input.CSV, "csv"
	if strings.TrimSpace(input.URL) != "" {
		source = "google_sheet"
		sheetURL, err := googleSheetsCSVURL(input.URL)
		if err != nil {
			return importTable{}, 400, err
		}
		client := &http.Client{Timeout: 30 * time.Second}
		response, err := client.Get(sheetURL)
		if err != nil {
			return importTable{}, 400, fmt.Errorf("could not download Google Sheet CSV")
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			log.Printf("import source=%s download_status=%d", source, response.StatusCode)
			return importTable{}, 400, fmt.Errorf("Google Sheets returned HTTP %d; publish the sheet to the web as CSV and try again", response.StatusCode)
		}
		data, _ := io.ReadAll(io.LimitReader(response.Body, 10<<20))
		raw = string(data)
	}
	raw = strings.TrimPrefix(raw, "\xef\xbb\xbf")
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = -1 // spreadsheets often export rows with uneven column counts
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	if firstLine, _, _ := strings.Cut(raw, "\n"); strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		reader.Comma = ';' // European Excel exports
	}
	records, err := reader.ReadAll()
	if err != nil {
		return importTable{}, 400, fmt.Errorf("could not read the CSV: %v", err)
	}
	// Skip blank lines before the header row.
	for len(records) > 0 && blankRow(records[0]) {
		records = records[1:]
	}
	rows := [][]string{}
	for _, row := range records[min(1, len(records)):] {
		if !blankRow(row) {
			rows = append(rows, row)
		}
	}
	if len(records) == 0 || len(rows) == 0 {
		return importTable{}, 400, fmt.Errorf("provide a CSV with a header row and at least one data row; Google Sheets must be published as CSV")
	}
	return importTable{headers: records[0], rows: rows}, 0, nil
}

func blankRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// columns resolves field -> column index, applying any manual mapping.
func (t importTable) columns(override map[string]string) map[string]int {
	mapping := lead.MapHeaders(t.headers)
	for field, header := range override {
		if !isImportField(field) {
			continue
		}
		delete(mapping, field)
		if header == "" {
			continue
		}
		for i, candidate := range t.headers {
			if strings.TrimSpace(strings.TrimPrefix(candidate, "\xef\xbb\xbf")) == strings.TrimSpace(header) {
				mapping[field] = i
				break
			}
		}
	}
	return mapping
}

func isImportField(field string) bool {
	for _, known := range lead.ImportFields {
		if known == field {
			return true
		}
	}
	return false
}

func (t importTable) inputs(mapping map[string]int, defaultSegment string) []lead.Input {
	first, last := lead.FirstLastColumns(t.headers)
	cell := func(row []string, index int) string {
		if index >= 0 && index < len(row) {
			return strings.TrimSpace(row[index])
		}
		return ""
	}
	value := func(row []string, field string) string {
		if index, ok := mapping[field]; ok {
			return cell(row, index)
		}
		return ""
	}
	inputs := make([]lead.Input, 0, len(t.rows))
	for _, row := range t.rows {
		input := lead.Input{
			Name: value(row, "name"), Organization: value(row, "organization"), Email: lead.CleanEmail(value(row, "email")),
			Phone: lead.CleanPhone(value(row, "phone")), Designation: value(row, "designation"), Address: value(row, "address"), Segment: value(row, "segment"),
		}
		if _, hasName := mapping["name"]; !hasName && (first >= 0 || last >= 0) {
			input.Name = strings.TrimSpace(cell(row, first) + " " + cell(row, last))
		}
		if input.Segment == "" {
			input.Segment = strings.TrimSpace(defaultSegment)
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func (t importTable) mappingByHeader(mapping map[string]int) map[string]string {
	named := map[string]string{}
	for _, field := range lead.ImportFields {
		named[field] = ""
		if index, ok := mapping[field]; ok {
			named[field] = strings.TrimSpace(strings.TrimPrefix(t.headers[index], "\xef\xbb\xbf"))
		}
	}
	return named
}

func (t importTable) ignoredHeaders(mapping map[string]int) []string {
	used := map[int]bool{}
	for _, index := range mapping {
		used[index] = true
	}
	first, last := lead.FirstLastColumns(t.headers)
	if _, hasName := mapping["name"]; !hasName {
		used[first], used[last] = true, true
	}
	ignored := []string{}
	for i, header := range t.headers {
		if !used[i] && strings.TrimSpace(header) != "" {
			ignored = append(ignored, strings.TrimSpace(strings.TrimPrefix(header, "\xef\xbb\xbf")))
		}
	}
	return ignored
}

func (s *Server) importPreviewHandler(w http.ResponseWriter, r *http.Request) {
	var input importRequest
	if decodeJSON(r, &input) != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	table, status, err := s.loadImportTable(input)
	if err != nil {
		writeError(w, err.Error(), status)
		return
	}
	mapping := table.columns(input.Mapping)
	inputs := table.inputs(mapping, input.Segment)
	sample := inputs[:min(len(inputs), 8)]
	headers := make([]string, len(table.headers))
	for i, header := range table.headers {
		headers[i] = strings.TrimSpace(strings.TrimPrefix(header, "\xef\xbb\xbf"))
	}
	withEmail := 0
	for _, in := range inputs {
		if in.Email != "" {
			withEmail++
		}
	}
	writeJSON(w, map[string]any{
		"headers": headers, "fields": lead.ImportFields, "mapping": table.mappingByHeader(mapping),
		"ignored": table.ignoredHeaders(mapping), "rows": len(inputs), "withEmail": withEmail, "sample": sample,
	}, 200)
}

func (s *Server) importHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input importRequest
	if decodeJSON(r, &input) != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	table, status, err := s.loadImportTable(input)
	if err != nil {
		writeError(w, err.Error(), status)
		return
	}
	mapping := table.columns(input.Mapping)
	if _, hasEmail := mapping["email"]; !hasEmail {
		if _, hasPhone := mapping["phone"]; !hasPhone {
			writeError(w, "No email or phone column was found. Choose the matching columns and try again.", 400)
			return
		}
	}
	results, err := s.leads.Import(table.inputs(mapping, input.Segment), 2)
	if err != nil {
		serverError(w, err)
		return
	}
	summary := map[string]int{"rows": len(results), "imported": 0, "duplicates": 0, "empty": 0, "invalid": 0}
	for _, result := range results {
		switch result.Status {
		case "added":
			summary["imported"]++
		case "duplicate":
			summary["duplicates"]++
		case "empty":
			summary["empty"]++
		case "invalid":
			summary["invalid"]++
		}
	}
	summary["skipped"] = summary["rows"] - summary["imported"]
	log.Printf("import complete rows=%d imported=%d duplicates=%d empty=%d invalid=%d", summary["rows"], summary["imported"], summary["duplicates"], summary["empty"], summary["invalid"])
	writeJSON(w, map[string]any{
		"rows": summary["rows"], "imported": summary["imported"], "skipped": summary["skipped"], "duplicates": summary["duplicates"],
		"empty": summary["empty"], "invalid": summary["invalid"], "mapping": table.mappingByHeader(mapping), "results": results,
	}, 200)
}

func googleSheetsCSVURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("Google Sheet URL must be a valid HTTPS URL")
	}
	if parsed.Host != "docs.google.com" || !strings.HasPrefix(parsed.Path, "/spreadsheets/d/") || strings.Contains(parsed.Path, "/d/e/") {
		return parsed.String(), nil
	}
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/spreadsheets/d/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", fmt.Errorf("could not find a spreadsheet ID in the Google Sheet URL")
	}
	query := url.Values{}
	if gid := parsed.Query().Get("gid"); gid != "" {
		query.Set("gid", gid)
	}
	query.Set("format", "csv")
	return "https://docs.google.com/spreadsheets/d/" + parts[0] + "/export?" + query.Encode(), nil
}
