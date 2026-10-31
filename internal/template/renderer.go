package render

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"giodevportal/internal/domain"
)

type Renderer struct {
	pages    map[string]*template.Template
	partials *template.Template
	fsys     fs.FS
}

func New(root string) (*Renderer, error) {
	r := &Renderer{pages: map[string]*template.Template{}, fsys: os.DirFS(root)}

	partials, err := template.New("partials").Funcs(FuncMap()).ParseFS(r.fsys, "partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse partials: %w", err)
	}
	r.partials = partials

	err = fs.WalkDir(r.fsys, "pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		t, err := template.New("base").Funcs(FuncMap()).ParseFS(r.fsys, "layouts/base.html", "partials/*.html", path)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		key := strings.TrimPrefix(path, "pages/")
		r.pages[key] = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(r.pages) == 0 {
		return nil, fmt.Errorf("no page templates found in %s", root)
	}
	return r, nil
}

func (r *Renderer) Render(w http.ResponseWriter, status int, page string, data map[string]any) error {
	t, ok := r.pages[filepath.ToSlash(page)]
	if !ok {
		return fmt.Errorf("page template not found: %s", page)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base.html", data); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := buf.WriteTo(w)
	return err
}

func (r *Renderer) Partial(w http.ResponseWriter, status int, name string, data any) error {
	var buf bytes.Buffer
	if err := r.partials.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := buf.WriteTo(w)
	return err
}

func FuncMap() template.FuncMap {
	return template.FuncMap{
		"date": domain.FormatDate,
		"dateInput": func(v string) string {
			if t, ok := domain.ParseTime(v); ok {
				return t.Format("2006-01-02")
			}
			return ""
		},
		"float":         func(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) },
		"datetime":      domain.FormatDateTime,
		"money":         domain.Money,
		"bytes":         domain.FormatBytes,
		"truncate":      domain.Truncate,
		"statusClass":   domain.StatusBadgeClass,
		"projectStatus": func(s string) string { return domain.ProjectStatusLabels[s] },
		"taskStatus":    func(s string) string { return domain.TaskStatusLabels[s] },
		"add":           func(a, b int) int { return a + b },
		"seq": func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = i
			}
			return out
		},
		"dict": dict,
		"default": func(def, v string) string {
			if strings.TrimSpace(v) == "" {
				return def
			}
			return v
		},
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"yesno": func(b bool) string {
			if b {
				return "Sí"
			}
			return "No"
		},
	}
}

func dict(values ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			continue
		}
		m[key] = values[i+1]
	}
	return m
}
