package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (a *App) ServeFile(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(chiURLParam(r, "*"), "/")
	if key == "" {
		http.NotFound(w, r)
		return
	}

	res, err := a.Q.GetResourceByKey(r.Context(), key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, ok := a.projectFor(w, r, res.ProjectID); !ok {
		return
	}

	url, err := a.Store.URL(r.Context(), key)
	if err != nil {
		http.Error(w, "No se pudo generar el enlace del archivo", http.StatusInternalServerError)
		return
	}
	if strings.HasPrefix(url, "http") {
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return
	}

	rc, err := a.Store.Open(r.Context(), key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()

	if res.ContentType != "" {
		w.Header().Set("Content-Type", res.ContentType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", res.FileName))
	_, _ = io.Copy(w, rc)
}
