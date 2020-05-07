// +build !solution

package dist

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"gitlab.com/slon/shad-go/distbuild/pkg/filecache"
	"gitlab.com/slon/shad-go/distbuild/pkg/scheduler"
)

type BuildService struct {
	logger      *zap.Logger
	fileCache   *filecache.Cache
	scheduler   *scheduler.Scheduler
	jobsWriters map[build.ID]api.StatusWriter // ADDED
}

func NewBuildService(logger *zap.Logger,
	fileCache *filecache.Cache,
	scheduler *scheduler.Scheduler) *BuildService {
	return &BuildService{
		logger:      logger,
		fileCache:   fileCache,
		scheduler:   scheduler,
		jobsWriters: make(map[build.ID]api.StatusWriter), // ADDED
	}
}

func (svc *BuildService) StartBuild(ctx context.Context, request *api.BuildRequest, w api.StatusWriter) error {
	svc.logger.Info("pkg/dist/coordinator.go StartBuild")
	// TODO: TopSort
	// TODO: цикл?
	// fmt.Printf("\n\n%v StartBuild start\n\n", time.Now())
	for _, job := range request.Graph.Jobs {
		jobSpec := api.JobSpec{
			Job:         job,
			SourceFiles: request.Graph.SourceFiles,
		}

		pendingJob := svc.scheduler.ScheduleJob(&jobSpec)

		buildStarted := &api.BuildStarted{
			ID:           pendingJob.Job.ID,
			MissingFiles: svc.scheduler.GetMissingFiles(pendingJob.Job.SourceFiles), // TODO
		}

		if pendingJob.Result != nil && pendingJob.Result.Error == nil {
			// TODO: шедулить джобу, результаты кэша брать из ответа воркера
			w.Started(buildStarted)
			w.Updated(&api.StatusUpdate{
				JobFinished: pendingJob.Result,
			})
			continue
		}
		_, exists := svc.jobsWriters[pendingJob.Job.ID]

		if !pendingJob.IsFinished && exists {
			fmt.Printf("\n\n%v Finished JobRuning start  %s\n\n", time.Now(), job.ID.String())
			<-pendingJob.Finished
			fmt.Printf("\n\n%v Finished JobRuning stop %s\n\n", time.Now(), job.ID.String())

			w.Started(buildStarted)

			// fmt.Println(">>>> pendingJob.Result", pendingJob.Result)
			w.Updated(&api.StatusUpdate{
				JobFinished: pendingJob.Result,
			})
			continue
		}
		svc.jobsWriters[pendingJob.Job.ID] = w // ADDED, TODO: lock

		w.Started(buildStarted)
		// svc.logger.Info("pkg/dist/coordinator.go StartBuild started, locked")         // ADDED
		fmt.Printf("\n\n%v Finished Started start  %s\n\n", time.Now(), job.ID.String())
		<-pendingJob.Finished
		fmt.Printf("\n\n%v Finished Started stop %s\n\n", time.Now(), job.ID.String())
		// svc.logger.Info("pkg/dist/coordinator.go StartBuild started, channel closed") // ADDED
	}
	// fmt.Printf("\n\n%v StartBuild end\n\n", time.Now())
	return nil
}

func (svc *BuildService) SignalBuild(ctx context.Context, buildID build.ID, signal *api.SignalRequest) (*api.SignalResponse, error) {
	svc.logger.Info("pkg/dist/coordinator.go SignalBuild")

	return &api.SignalResponse{}, nil
}

func (svc *BuildService) Heartbeat(ctx context.Context, req *api.HeartbeatRequest) (*api.HeartbeatResponse, error) {
	svc.logger.Info("pkg/dist/coordinator.go Heartbeat")
	fmt.Printf("\n\n%v Heartbeat start %s\n\n", time.Now(), req.WorkerID)

	// println("!!! HeartbeatRequest")
	// spew.Dump(req.FinishedJob)
	for _, jobResult := range req.FinishedJob {
		// fmt.Println("svc.jobsWriters", svc.jobsWriters)
		statusWriter := svc.jobsWriters[jobResult.ID] // ADDED TODO: lock, exists check
		fmt.Println("===== Heartbeat jobResult", req.WorkerID, jobResult)
		statusWriter.Updated(&api.StatusUpdate{ // ADDED
			JobFinished: &jobResult,
		})
		svc.scheduler.OnJobComplete(req.WorkerID, jobResult.ID, &jobResult) // ADDED (moved)
	}

	fmt.Printf("\n\n%v Heartbeat before PickJob, %s\n\n", time.Now(), req.WorkerID)
	pendingJob := svc.scheduler.PickJob(ctx, req.WorkerID)
	fmt.Printf("\n\n%v Heartbeat after PickJob, %s\n\n", time.Now(), req.WorkerID)

	if pendingJob == nil {
		return &api.HeartbeatResponse{}, nil
	}

	pendingJob.Job.Artifacts = make(map[build.ID]api.WorkerID)
	for _, artifactID := range pendingJob.Job.Deps {
		workerID, exists := svc.scheduler.LocateArtifact(artifactID)
		if !exists {
			continue
		}
		pendingJob.Job.Artifacts[artifactID] = workerID
	}

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

	buildHandler := api.NewBuildService(log, buildSvc)
	buildHandler.Register(mux)
	heartbeatHandler := api.NewHeartbeatHandler(log, buildSvc)
	heartbeatHandler.Register(mux)
	filecacheHandler := filecache.NewHandler(log, fileCache)
	filecacheHandler.Register(mux)
	locateArtifacHandler := NewLocateArtifactHandler(log, schedulerSvc)
	locateArtifacHandler.Register(mux)

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
