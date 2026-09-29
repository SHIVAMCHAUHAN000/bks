package lead

import (
	"regexp"
	"strings"
)

// ImportFields are the lead fields a CSV column can be mapped to, in display order.
var ImportFields = []string{"name", "organization", "email", "phone", "designation", "address", "segment"}

var headerAliases = map[string][]string{
	"name":         {"name", "full name", "contact name", "contact person", "person name", "client name", "lead name", "customer name", "person", "poc", "poc name", "spoc", "owner name"},
	"organization": {"organization", "organisation", "org", "org name", "organization name", "organisation name", "company", "company name", "firm", "firm name", "business", "business name", "institution", "institute", "account", "account name", "employer", "school", "college", "hospital"},
	"email":        {"email", "e mail", "email id", "e mail id", "email address", "e mail address", "mail", "mail id", "emailid", "work email", "official email", "business email"},
	"phone":        {"phone", "phone number", "phone no", "number", "mobile", "mobile number", "mobile no", "contact", "contact number", "contact no", "cell", "cell phone", "telephone", "tel", "whatsapp", "whatsapp number", "ph", "ph no", "landline"},
	"designation":  {"designation", "title", "job title", "position", "role", "job role", "job position", "post"},
	"address":      {"address", "full address", "office address", "location", "city", "street", "street address", "area", "address line 1"},
	"segment":      {"segment", "category", "industry", "sector", "type", "group", "vertical"},
}

var nonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

// NormalizeHeader lowercases a heading and removes punctuation and BOM characters.
func NormalizeHeader(value string) string {
	value = strings.TrimPrefix(value, "﻿")
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.TrimSpace(nonAlphaNumeric.ReplaceAllString(value, " "))
}

// MapHeaders guesses which CSV column holds each lead field. It returns field -> column index.
func MapHeaders(headers []string) map[string]int {
	normalized := make([]string, len(headers))
	for i, header := range headers {
		normalized[i] = NormalizeHeader(header)
	}
	mapping := map[string]int{}
	used := map[int]bool{}
	// Exact alias matches first, in alias priority order.
	for _, field := range ImportFields {
		for _, alias := range headerAliases[field] {
			for i, header := range normalized {
				if !used[i] && header == alias {
					mapping[field], used[i] = i, true
					break
				}
			}
			if _, ok := mapping[field]; ok {
				break
			}
		}
	}
	// Then headings that contain an alias as a whole word, e.g. "Primary Email".
	for _, field := range ImportFields {
		if _, ok := mapping[field]; ok {
			continue
		}
		for i, header := range normalized {
			if used[i] {
				continue
			}
			for _, alias := range headerAliases[field] {
				if len(alias) > 3 && containsWords(header, alias) {
					mapping[field], used[i] = i, true
					break
				}
			}
			if _, ok := mapping[field]; ok {
				break
			}
		}
	}
	return mapping
}

func containsWords(header, alias string) bool {
	return strings.Contains(" "+header+" ", " "+alias+" ")
}

// FirstLastColumns finds separate first/last name columns when there is no full-name column.
func FirstLastColumns(headers []string) (first, last int) {
	first, last = -1, -1
	for i, header := range headers {
		switch NormalizeHeader(header) {
		case "first name", "firstname", "given name", "fname":
			first = i
		case "last name", "lastname", "surname", "family name", "lname":
			last = i
		}
	}
	return first, last
}

var emailSeparator = regexp.MustCompile(`[;,/\s]+`)

// CleanEmail keeps the first address when a cell contains several.
func CleanEmail(value string) string {
	value = strings.TrimSpace(strings.Trim(value, "<>\"'"))
	value = strings.TrimPrefix(strings.ToLower(value), "mailto:")
	for _, part := range emailSeparator.Split(value, -1) {
		if strings.Contains(part, "@") {
			return strings.Trim(part, "<>\"'.")
		}
	}
	return value
}

// CleanPhone removes spreadsheet artefacts such as a trailing ".0" or a leading apostrophe.
func CleanPhone(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "'"))
	if strings.HasSuffix(value, ".0") && !strings.ContainsAny(value, " -+()") {
		value = strings.TrimSuffix(value, ".0")
	}
	return value
}
