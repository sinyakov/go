package dist

import (
	"encoding/json"
	"net/http"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"

	"gitlab.com/slon/shad-go/distbuild/pkg/scheduler"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"go.uber.org/zap"
)

type Handler struct {
	logger       *zap.Logger
	schedulerSvc *scheduler.Scheduler
}

type LocateArtifactResponse struct {
	WorkerID api.WorkerID
	Exists   bool
}

func NewLocateArtifactHandler(l *zap.Logger, schedulerSvc *scheduler.Scheduler) *Handler {
	return &Handler{
		logger:       l,
		schedulerSvc: schedulerSvc,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/locate_artifact", func(w http.ResponseWriter, r *http.Request) {
		h.logger.Info("pkg/dist/locate_artifact_handler.go /locate_artifact")
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
		workerID, exists := h.schedulerSvc.LocateArtifact(id)
		data, err := json.Marshal(LocateArtifactResponse{
			WorkerID: workerID,
			Exists:   exists,
		})
		if err != nil {
			h.logger.Error("pkg/dist/locate_artifact_handler.go /locate_artifact server error")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(data)
		if err != nil {
			h.logger.Error("pkg/dist/locate_artifact_handler.go /locate_artifact server error")
			_, errWrite := w.Write([]byte(err.Error()))
			if errWrite != nil {
				return
			}
		}
	})
}
