// +build !solution

package artifact

import (
	"net/http"

	"gitlab.com/slon/shad-go/distbuild/pkg/tarstream"

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
	mux.HandleFunc("/artifact", func(w http.ResponseWriter, r *http.Request) {
		queries := r.URL.Query()
		idStr := queries.Get("id")

		if idStr == "" {
			http.Error(w, "no id param", http.StatusBadRequest)
			return
		}

		var id build.ID
		id.UnmarshalText([]byte(idStr))

		path, unlock, err := h.cache.Get(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer unlock()

		w.Header().Set("Content-Type", "application-x/tar")
		w.WriteHeader(http.StatusOK)
		err = tarstream.Send(path, w)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
