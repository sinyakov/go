// +build !solution

package dist

import (
	"context"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"gitlab.com/slon/shad-go/distbuild/pkg/filecache"
	"gitlab.com/slon/shad-go/distbuild/pkg/scheduler"
)

type BuildService struct {
	logger    *zap.Logger
	fileCache *filecache.Cache
	scheduler *scheduler.Scheduler
}

func NewBuildService(logger *zap.Logger,
	fileCache *filecache.Cache,
	scheduler *scheduler.Scheduler) *BuildService {
	return &BuildService{
		logger:    logger,
		fileCache: fileCache,
		scheduler: scheduler,
	}
}

func (svc *BuildService) StartBuild(ctx context.Context, request *api.BuildRequest, w api.StatusWriter) error {
	svc.logger.Info("pkg/dist/coordinator.go StartBuild")

	// TODO: цикл?
	jobSpec := api.JobSpec{
		Job:         request.Graph.Jobs[0],
		SourceFiles: request.Graph.SourceFiles,
	}

	pendingJob := svc.scheduler.ScheduleJob(&jobSpec)

	buildStarted := &api.BuildStarted{
		ID: pendingJob.Job.ID,
	}

	w.Started(buildStarted)

	return nil
}

func (svc *BuildService) SignalBuild(ctx context.Context, buildID build.ID, signal *api.SignalRequest) (*api.SignalResponse, error) {
	svc.logger.Info("pkg/dist/coordinator.go SignalBuild")

	return &api.SignalResponse{}, nil
}

func (svc *BuildService) Heartbeat(ctx context.Context, req *api.HeartbeatRequest) (*api.HeartbeatResponse, error) {
	svc.logger.Info("pkg/dist/coordinator.go Heartbeat")
	println("!!! HeartbeatRequest")
	spew.Dump(req)
	for _, jobResult := range req.FinishedJob {
		svc.scheduler.OnJobComplete(req.WorkerID, jobResult.ID, &jobResult)
	}

	pendingJob := svc.scheduler.PickJob(ctx, req.WorkerID)
	jobsToRun := make(map[build.ID]api.JobSpec)
	jobsToRun[pendingJob.Job.ID] = *pendingJob.Job

	return &api.HeartbeatResponse{
		JobsToRun: jobsToRun,
	}, nil
}

type Coordinator struct {
	logger    *zap.Logger
	fileCache *filecache.Cache
	scheduler *scheduler.Scheduler
	mux       *http.ServeMux
}

var defaultConfig = scheduler.Config{
	CacheTimeout: time.Millisecond * 10,
	DepsTimeout:  time.Millisecond * 100,
}

func NewCoordinator(
	log *zap.Logger,
	fileCache *filecache.Cache,
) *Coordinator {
	schedulerSvc := scheduler.NewScheduler(log, defaultConfig)
	buildSvc := NewBuildService(log, fileCache, schedulerSvc)
	mux := http.NewServeMux()
	// buildHandler *api.BuildHandler
	// heartbeatHandler *api.HeartbeatHandler

	buildHandler := api.NewBuildService(log, buildSvc)
	buildHandler.Register(mux)
	heartbeatHandler := api.NewHeartbeatHandler(log, buildSvc)
	heartbeatHandler.Register(mux)

	return &Coordinator{
		logger:    log,
		fileCache: fileCache,
		scheduler: schedulerSvc,
		mux:       mux,
	}
}

func (c *Coordinator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mux.ServeHTTP(w, r)
}
