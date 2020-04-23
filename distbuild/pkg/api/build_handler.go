// +build !solution

package api

import (
	"encoding/json"
	"net/http"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"go.uber.org/zap"
)

func NewBuildService(l *zap.Logger, s Service) *BuildHandler {
	return &BuildHandler{
		logger:  l,
		service: s,
	}
}

type BuildHandler struct {
	logger  *zap.Logger
	service Service
}

type statusWriter struct {
	w              http.ResponseWriter
	flusher        http.Flusher
	isHeaderWrited bool
}

func NewStatusWriter(w http.ResponseWriter) *statusWriter {
	f := w.(http.Flusher)

	return &statusWriter{
		w:              w,
		flusher:        f,
		isHeaderWrited: false,
	}
}

func (sw *statusWriter) Started(rsp *BuildStarted) error {
	if !sw.isHeaderWrited {
		sw.w.Header().Set("Content-Type", "application/json")
		sw.w.WriteHeader(http.StatusOK)
		sw.isHeaderWrited = true
	}
	err := json.NewEncoder(sw.w).Encode(rsp)
	sw.flusher.Flush()
	return err
}

func (sw *statusWriter) Updated(update *StatusUpdate) error {
	err := json.NewEncoder(sw.w).Encode(update)
	sw.flusher.Flush()
	return err
}

func (h *BuildHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/build", func(w http.ResponseWriter, r *http.Request) {
		var buildRequest BuildRequest
		err := json.NewDecoder(r.Body).Decode(&buildRequest)
		if err != nil {
			h.logger.Error("/build request JSON decode error", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		stWriter := NewStatusWriter(w)

		err = h.service.StartBuild(r.Context(), &buildRequest, stWriter)
		if err != nil {
			h.logger.Error("/build SignalBuild error", zap.Error(err))
			if !stWriter.isHeaderWrited {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				stWriter.Updated(&StatusUpdate{
					BuildFailed: &BuildFailed{
						Error: err.Error(),
					},
				})
			}
		}
	})

	mux.HandleFunc("/signal", func(w http.ResponseWriter, r *http.Request) {
		queries := r.URL.Query()
		idStr := queries.Get("build_id")

		if idStr == "" {
			http.Error(w, "no id param", http.StatusBadRequest)
			return
		}

		var id build.ID
		id.UnmarshalText([]byte(idStr))

		var signalRequest SignalRequest
		err := json.NewDecoder(r.Body).Decode(&signalRequest)
		if err != nil {
			h.logger.Error("/signal request JSON decode error", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := h.service.SignalBuild(r.Context(), id, &signalRequest)
		if err != nil {
			h.logger.Error("/signal SignalBuild error", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(resp)
		if err != nil {
			h.logger.Error("/signal JSON marshaling failed", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, err = w.Write(data)
		if err != nil {
			h.logger.Error("/signal server error")
			w.Write([]byte(err.Error()))
		}
	})
}
