// +build !solution

package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/artifact"
	"gitlab.com/slon/shad-go/distbuild/pkg/filecache"
)

type Worker struct {
	workerID            api.WorkerID
	coordinatorEndpoint string
	logger              *zap.Logger
	fileCache           *filecache.Cache
	artifacts           *artifact.Cache
}

func New(
	workerID api.WorkerID,
	coordinatorEndpoint string,
	log *zap.Logger,
	fileCache *filecache.Cache,
	artifacts *artifact.Cache,
) *Worker {
	return &Worker{
		workerID:            workerID,
		coordinatorEndpoint: coordinatorEndpoint,
		logger:              log,
		fileCache:           fileCache,
		artifacts:           artifacts,
	}
}

func (w *Worker) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w.logger.Info("pkg/worker/worker.go ServeHTTP")
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("pkg/worker/worker.go Run")
	heartbeatClient := api.NewHeartbeatClient(w.logger, w.coordinatorEndpoint)
	var finishedJob []api.JobResult
	for {
		fmt.Println("finishedJob", finishedJob)
		resp, err := heartbeatClient.Heartbeat(ctx, &api.HeartbeatRequest{
			WorkerID:    w.workerID,
			FinishedJob: finishedJob,
		})

		if errors.Is(err, context.Canceled) {
			w.logger.Info("pkg/worker/worker.go context canceled")
			return err
		}

		if err != nil {
			w.logger.Error("pkg/worker/worker.go Heartbeat", zap.Error(err))
			continue
		}

		fmt.Println("pkg/worker/worker.go Run HeartbeatResponse, JobsToRun")

		finishedJob = []api.JobResult{}
		for _, jobSpec := range resp.JobsToRun {
			// spew.Dump(jobID, jobSpec.Cmds)
			for _, cmdWithArgs := range jobSpec.Cmds {
				fmt.Println(cmdWithArgs.Exec)
				cmd := exec.Command(cmdWithArgs.Exec[0], cmdWithArgs.Exec[1:]...)
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err := cmd.Run()
				var errStr *string
				exitCode := 0
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				}
				if err != nil {
					str := err.Error()
					errStr = &str
				}
				finishedJob = append(finishedJob, api.JobResult{
					ID:       jobSpec.ID,
					Stdout:   stdout.Bytes(),
					Stderr:   stderr.Bytes(),
					ExitCode: exitCode,
					Error:    errStr,
				})
			}
		}
	}
}
