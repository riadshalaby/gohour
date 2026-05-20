package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"fmtHours": func(value float64) string {
			return fmt.Sprintf("%.2f", value)
		},
		"fmtDelta": func(value float64) string {
			return fmt.Sprintf("%+.2f", value)
		},
		"isZeroDelta": func(value float64) bool {
			return math.Abs(value) < 0.0001
		},
		"toMins": func(hours float64) int {
			return int(math.Round(hours * 60))
		},
		// dayOffset returns the ISO date string offset by n days from the given ISO date.
		// Used in day.html for prev/next navigation links.
		"dayOffset": func(isoDate string, n int) string {
			t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(isoDate), time.Local)
			if err != nil {
				return isoDate
			}
			return t.AddDate(0, 0, n).Format("2006-01-02")
		},
	}
}

func renderTemplate(w http.ResponseWriter, pageTemplate string, data any) error {
	tmpl, err := template.New("base.html").Funcs(templateFuncMap()).ParseFS(
		templateFS, "templates/base.html", "templates/"+pageTemplate,
	)
	if err != nil {
		return fmt.Errorf("parse template %s: %w", pageTemplate, err)
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		return fmt.Errorf("render template %s: %w", pageTemplate, err)
	}
	return nil
}

// renderPartialTemplate renders an HTML partial (no base wrapper).
// The partial template must define a template named "partial".
func renderPartialTemplate(w http.ResponseWriter, partialTemplate string, data any) error {
	tmpl, err := template.New("partial").Funcs(templateFuncMap()).ParseFS(
		templateFS, "templates/"+partialTemplate,
	)
	if err != nil {
		return fmt.Errorf("parse partial template %s: %w", partialTemplate, err)
	}
	if err := tmpl.ExecuteTemplate(w, "partial", data); err != nil {
		return fmt.Errorf("render partial template %s: %w", partialTemplate, err)
	}
	return nil
}

func writePartialTableError(w http.ResponseWriter, statusCode int, colspan int, message string) {
	if colspan < 1 {
		colspan = 1
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	escaped := template.HTMLEscapeString(message)
	_, _ = fmt.Fprintf(w, `<tr><td colspan="%d"><div class="dialog-error">%s</div></td></tr>`, colspan, escaped)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain a single JSON object")
	}
	return nil
}

func formatRefreshTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
