// +build !solution

package filecache

import (
	"io"
	"net/http"
	"os"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"go.uber.org/zap"
)

type Handler struct {
	logger *zap.Logger
	cache  *Cache
}

func NewHandler(l *zap.Logger, c *Cache) *Handler {
	return &Handler{
		logger: l,
		cache:  c,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		queries := r.URL.Query()
		idStr := queries.Get("id")

		if idStr == "" {
			http.Error(w, "no id param", http.StatusBadRequest)
			return
		}

		var id build.ID
		err := id.UnmarshalText([]byte(idStr))
		if err != nil {
			return
		}

		if r.Method == "GET" {
			h.ServeGetFile(w, r, id)
		} else if r.Method == "PUT" {
			h.ServePutFile(w, r, id)
		}
	})
}

func (h *Handler) ServeGetFile(w http.ResponseWriter, r *http.Request, id build.ID) {
	filePath, unlock, err := h.cache.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer unlock()

	f, err := os.OpenFile(filePath, os.O_RDONLY, 0755)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)

	_, _ = io.Copy(w, f)
}

func (h *Handler) ServePutFile(w http.ResponseWriter, r *http.Request, id build.ID) {
	wc, abort, err := h.cache.Write(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(wc, r.Body)

	if err != nil {
		errAbort := abort()
		if errAbort != nil {
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	r.Body.Close()

	w.Header().Set("Content-Type", "plain/text")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("OK"))
	if err != nil {
		return
	}
}
