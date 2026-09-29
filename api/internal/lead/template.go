package lead

import (
	"html"
	"regexp"
	"sort"
	"strings"
)

// Placeholders are written as {{name}} or {{name|fallback}}.
var placeholderPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_]+)\s*(?:\|([^}]*))?\}\}`)

// TemplateFields lists the placeholders the campaign editor offers.
var TemplateFields = []string{"name", "first_name", "organization", "designation", "address", "email", "phone", "segment"}

var fieldAliases = map[string]string{"company": "organization", "organisation": "organization", "title": "designation", "firstname": "first_name", "fullname": "name"}

func (l Lead) fieldValue(field string) (string, bool) {
	field = strings.ToLower(field)
	if alias, ok := fieldAliases[field]; ok {
		field = alias
	}
	switch field {
	case "name":
		return strings.TrimSpace(l.Name), true
	case "first_name":
		return FirstName(l.Name), true
	case "organization":
		return strings.TrimSpace(l.Organization), true
	case "designation":
		return strings.TrimSpace(l.Designation), true
	case "address":
		return strings.TrimSpace(l.Address), true
	case "email":
		return l.Email, true
	case "phone":
		return l.Phone, true
	case "segment":
		return l.Segment, true
	}
	return "", false
}

// FirstName returns the first word of a name, skipping common honorifics.
func FirstName(name string) string {
	parts := strings.Fields(name)
	for len(parts) > 1 {
		switch strings.TrimSuffix(strings.ToLower(parts[0]), ".") {
		case "mr", "mrs", "ms", "miss", "dr", "prof", "shri", "smt", "sri", "er":
			parts = parts[1:]
			continue
		}
		break
	}
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// Render fills placeholders with the lead's values. Missing values use the fallback, or an empty string.
func Render(template string, l Lead) string {
	out := placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		groups := placeholderPattern.FindStringSubmatch(match)
		value, known := l.fieldValue(groups[1])
		if !known {
			return match
		}
		if value == "" {
			return strings.TrimSpace(groups[2])
		}
		return value
	})
	// Tidy spacing left behind by empty placeholders, e.g. "Hi ," -> "Hi,".
	out = strings.ReplaceAll(out, " ,", ",")
	return out
}

// UnknownPlaceholders reports placeholders that will not be replaced.
func UnknownPlaceholders(templates ...string) []string {
	seen := map[string]bool{}
	for _, template := range templates {
		for _, groups := range placeholderPattern.FindAllStringSubmatch(template, -1) {
			if _, known := (Lead{}).fieldValue(groups[1]); !known {
				seen[groups[1]] = true
			}
		}
	}
	unknown := make([]string, 0, len(seen))
	for name := range seen {
		unknown = append(unknown, name)
	}
	sort.Strings(unknown)
	return unknown
}

// PlainToHTML converts a plain-text message into simple, spam-filter friendly HTML.
func PlainToHTML(text string) string {
	paragraphs := strings.Split(strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n"), "\n\n")
	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:1.55;color:#222">`)
	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) == "" {
			continue
		}
		b.WriteString(`<p style="margin:0 0 14px">`)
		b.WriteString(strings.ReplaceAll(linkify(html.EscapeString(paragraph)), "\n", "<br>"))
		b.WriteString(`</p>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

var urlPattern = regexp.MustCompile(`https?://[^\s<]+`)

func linkify(escaped string) string {
	return urlPattern.ReplaceAllStringFunc(escaped, func(url string) string {
		trimmed := strings.TrimRight(url, ".,;:)")
		return `<a href="` + trimmed + `">` + trimmed + `</a>` + url[len(trimmed):]
	})
}
