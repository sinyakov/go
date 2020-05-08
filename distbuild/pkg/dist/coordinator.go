// +build !solution

package dist

import (
	"context"
	"fmt"
	"net/http"
	"sync"
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
	mutex       *sync.Mutex
}

func NewBuildService(logger *zap.Logger,
	fileCache *filecache.Cache,
	scheduler *scheduler.Scheduler) *BuildService {
	return &BuildService{
		logger:      logger,
		fileCache:   fileCache,
		scheduler:   scheduler,
		jobsWriters: make(map[build.ID]api.StatusWriter), // ADDED
		mutex:       &sync.Mutex{},
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
			fmt.Printf("DDD: уже готовая джоба, записан результат %v\n", pendingJob.Result)
			continue
		}

		_, exists := svc.jobsWriters[pendingJob.Job.ID]
		if !pendingJob.IsFinished && exists {
			w.Started(buildStarted)

			<-pendingJob.Finished
			// svc.mutex.Lock()
			res := *pendingJob.Result
			fmt.Printf("DDD: была в работе, доделалась, записан результат: %v\n", res)
			w.Updated(&api.StatusUpdate{
				JobFinished: &res,
			})
			// svc.mutex.Unlock()
			continue
		}

		svc.jobsWriters[pendingJob.Job.ID] = w // ADDED, TODO: lock

		w.Started(buildStarted)
		// svc.logger.Info("pkg/dist/coordinator.go StartBuild started, locked")         // ADDED
		<-pendingJob.Finished
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
	fmt.Printf("\n%v Heartbeat start %s\n", time.Now(), req.WorkerID)

	// println("!!! HeartbeatRequest")
	// spew.Dump(req.FinishedJob)
	for _, jobResult := range req.FinishedJob {
		// fmt.Println("svc.jobsWriters", svc.jobsWriters)
		svc.mutex.Lock()
		statusWriter := svc.jobsWriters[jobResult.ID] // ADDED TODO: lock, exists check
		fmt.Printf("DDD: выполнена на воркере, записан результат: %v\n", &jobResult)
		statusWriter.Updated(&api.StatusUpdate{ // ADDED
			JobFinished: &jobResult,
		})
		svc.scheduler.OnJobComplete(req.WorkerID, jobResult.ID, &jobResult) // ADDED (moved)
		svc.mutex.Unlock()
	}

	pendingJob := svc.scheduler.PickJob(ctx, req.WorkerID)
	if pendingJob != nil {
		fmt.Printf("%v Heartbeat PickJob new job picked, %s\n", time.Now(), req.WorkerID)
	}

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
