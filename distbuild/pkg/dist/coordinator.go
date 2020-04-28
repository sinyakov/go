// +build !solution

package dist

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"gitlab.com/slon/shad-go/distbuild/pkg/filecache"
	"gitlab.com/slon/shad-go/distbuild/pkg/scheduler"
)

type Coordinator struct {
	logger    *zap.Logger
	fileCache *filecache.Cache
	scheduler *scheduler.Scheduler
}

var defaultConfig = scheduler.Config{
	CacheTimeout: time.Millisecond * 10,
	DepsTimeout:  time.Millisecond * 100,
}

func NewCoordinator(
	log *zap.Logger,
	fileCache *filecache.Cache,
) *Coordinator {
	return &Coordinator{
		logger:    log,
		fileCache: fileCache,
		scheduler: scheduler.NewScheduler(log, defaultConfig),
	}
}

func (c *Coordinator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "/coordinator/build" {
		c.StartBuild(w, r)
	}

	if r.RequestURI == "/coordinator/heartbeat" {
		c.Heartbeat(w, r)
	}
}

func (c *Coordinator) StartBuild(w http.ResponseWriter, r *http.Request) {
	c.logger.Info("pkg/dist/coordinator.go ServeHTTP", zap.String("RequestURI", r.RequestURI))
	var buildRequest api.BuildRequest

	err := json.NewDecoder(r.Body).Decode(&buildRequest)
	if err != nil {
		c.logger.Error("pkg/dist/coordinator.go StartBuildDecode", zap.String("RequestURI", r.RequestURI))
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	fmt.Println("pkg/dist/coordinator.go StartBuild buildRequest")
	// spew.Dump(buildRequest)

	// TODO: цикл?
	jobSpec := api.JobSpec{
		Job:         buildRequest.Graph.Jobs[0],
		SourceFiles: buildRequest.Graph.SourceFiles,
	}

	pendingJob := c.scheduler.ScheduleJob(&jobSpec)

	data, err := json.Marshal(api.BuildStarted{
		ID: pendingJob.Job.ID,
		// MissingFiles
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)

	// fmt.Println("pkg/dist/coordinator.go StartBuild pendingJob")
	// spew.Dump(pendingJob)
}

func (c *Coordinator) Heartbeat(w http.ResponseWriter, r *http.Request) {
	c.logger.Info("pkg/dist/coordinator.go ServeHTTP", zap.String("RequestURI", r.RequestURI))
	var heartbeatRequest api.HeartbeatRequest

	err := json.NewDecoder(r.Body).Decode(&heartbeatRequest)
	if err != nil {
		c.logger.Info("pkg/dist/coordinator.go Heartbeat Decode", zap.String("RequestURI", r.RequestURI))
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	fmt.Println("pkg/dist/coordinator.go heartbeatRequest")
	spew.Dump(heartbeatRequest)

	pendingJob := c.scheduler.PickJob(r.Context(), heartbeatRequest.WorkerID)
	JobsToRun := make(map[build.ID]api.JobSpec)
	JobsToRun[pendingJob.Job.ID] = *pendingJob.Job
	data, err := json.Marshal(api.HeartbeatResponse{
		JobsToRun: JobsToRun,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)

	// fmt.Println("pkg/dist/coordinator.go pendingJob")
	// spew.Dump(pendingJob)

}
