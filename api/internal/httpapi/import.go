package httpapi

import (
	"encoding/csv"
	"io"
	"automationtool/api/internal/lead"
	"net/http"
	"strings"
)

func (s *Server) importHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CSV string `json:"csv"`
		URL string `json:"url"`
	}
	if decodeJSON(r, &input) != nil {
		writeJSON(w, map[string]string{"error": "invalid JSON"}, 400)
		return
	}
	raw := input.CSV
	if input.URL != "" {
		response, err := http.Get(input.URL)
		if err != nil {
			writeJSON(w, map[string]string{"error": "could not download sheet"}, 400)
			return
		}
		defer response.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(response.Body, 10<<20))
		raw = string(data)
	}
	records, err := csv.NewReader(strings.NewReader(raw)).ReadAll()
	if err != nil || len(records) < 2 {
		writeJSON(w, map[string]string{"error": "provide a CSV with headers and a row"}, 400)
		return
	}
	headers := map[string]int{}
	for i, name := range records[0] {
		headers[strings.ToLower(strings.TrimSpace(name))] = i
	}
	value := func(row []string, name string) string {
		if index, ok := headers[name]; ok && index < len(row) {
			return strings.TrimSpace(row[index])
		}
		return ""
	}
	imported := 0
	for _, row := range records[1:] {
		input := lead.Input{Name: value(row, "name"), Email: value(row, "email"), Phone: value(row, "phone"), Category: value(row, "category"), Subcategory: value(row, "subcategory")}
		if input.Email == "" && input.Phone == "" {
			continue
		}
		if _, err := s.leads.Create(input); err == nil {
			imported++
		}
	}
	writeJSON(w, map[string]int{"imported": imported, "rows": len(records) - 1}, 200)
}
