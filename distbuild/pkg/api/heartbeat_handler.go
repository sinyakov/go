// +build !solution

package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

type HeartbeatHandler struct {
	logger  *zap.Logger
	service HeartbeatService
}

func NewHeartbeatHandler(l *zap.Logger, s HeartbeatService) *HeartbeatHandler {
	return &HeartbeatHandler{
		logger:  l,
		service: s,
	}
}

func (h *HeartbeatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var heartbeatRequest HeartbeatRequest
		err := json.NewDecoder(r.Body).Decode(&heartbeatRequest)
		if err != nil {
			h.logger.Error("/heartbeat body parse", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp, err := h.service.Heartbeat(r.Context(), &heartbeatRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := json.Marshal(resp)
		if err != nil {
			h.logger.Error("/heartbeat JSON marshaling failed", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, err = w.Write(data)
		if err != nil {
			h.logger.Error("/heartbeat server error")
			w.Write([]byte(err.Error()))
		}
	})
}
