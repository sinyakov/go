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

type StatusWriterImpl struct {
	w              http.ResponseWriter
	flusher        http.Flusher
	isHeaderWrited bool
	encoder        *json.Encoder
}

func NewStatusWriter(w http.ResponseWriter) *StatusWriterImpl {
	f := w.(http.Flusher)

	return &StatusWriterImpl{
		w:              w,
		flusher:        f,
		isHeaderWrited: false,
		encoder:        json.NewEncoder(w),
	}
}

func (sw *StatusWriterImpl) Started(rsp *BuildStarted) error {
	if !sw.isHeaderWrited {
		sw.w.Header().Set("Content-Type", "application/json")
		sw.w.WriteHeader(http.StatusOK)
		sw.isHeaderWrited = true
	}
	err := json.NewEncoder(sw.w).Encode(rsp)
	sw.flusher.Flush()
	return err
}

func (sw *StatusWriterImpl) Updated(update *StatusUpdate) error {
	err := sw.encoder.Encode(update)
	sw.flusher.Flush()
	return err
}

func (h *BuildHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/build", func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("pkg/api/build_handler.go /build")

		var buildRequest BuildRequest
		err := json.NewDecoder(r.Body).Decode(&buildRequest)
		if err != nil {
			h.logger.Error("pkg/api/build_handler.go /build request JSON decode error", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		stWriter := NewStatusWriter(w)

		h.logger.Info("pkg/api/build_handler.go /build h.service.StartBuild started")
		err = h.service.StartBuild(r.Context(), &buildRequest, stWriter)
		h.logger.Info("pkg/api/build_handler.go /build h.service.StartBuild completed")
		if err != nil {
			h.logger.Error("pkg/api/build_handler.go /build error", zap.Error(err))
			if !stWriter.isHeaderWrited {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				errWriter := stWriter.Updated(&StatusUpdate{
					BuildFailed: &BuildFailed{
						Error: err.Error(),
					},
				})
				if errWriter != nil {
					return
				}
			}
		}
	})

	mux.HandleFunc("/signal", func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("pkg/api/build_handler.go /signal")

		queries := r.URL.Query()
		idStr := queries.Get("build_id")

		if idStr == "" {
			http.Error(w, "no id param", http.StatusBadRequest)
			return
		}

		var id build.ID
		err := id.UnmarshalText([]byte(idStr))
		if err != nil {
			return
		}
		var signalRequest SignalRequest
		err = json.NewDecoder(r.Body).Decode(&signalRequest)
		if err != nil {
			h.logger.Error("pkg/api/build_handler.go /signal request JSON decode error", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := h.service.SignalBuild(r.Context(), id, &signalRequest)
		if err != nil {
			h.logger.Error("pkg/api/build_handler.go /signal SignalBuild error", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(resp)
		if err != nil {
			h.logger.Error("pkg/api/build_handler.go /signal JSON marshaling failed", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, err = w.Write(data)
		if err != nil {
			h.logger.Error("pkg/api/build_handler.go /signal server error")
			_, errWrite := w.Write([]byte(err.Error()))
			if errWrite != nil {
				return
			}
		}
	})
}
