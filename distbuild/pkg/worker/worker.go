// +build !solution

package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	gopath "path"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/artifact"
	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"gitlab.com/slon/shad-go/distbuild/pkg/filecache"
)

type Worker struct {
	workerID            api.WorkerID
	coordinatorEndpoint string
	logger              *zap.Logger
	fileCache           *filecache.Cache
	artifacts           *artifact.Cache
	filecacheClient     *filecache.Client
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
		filecacheClient:     filecache.NewClient(log, coordinatorEndpoint),
	}
}

func (w *Worker) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w.logger.Info("pkg/worker/worker.go ServeHTTP")
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func (w *Worker) Run(ctx context.Context) error {
	// TODO: сохранять результаты работы
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
			tmpSourceDir := os.TempDir()

			for fileID, fileNameWithPath := range jobSpec.SourceFiles {
				path, unlock, err := w.fileCache.Get(fileID)
				if err == nil {
					unlock()
					continue
				}
				err = w.filecacheClient.Download(ctx, w.fileCache, fileID)
				if err != nil {
					fmt.Println("Download", err)
					return err
				}
				path, unlock, err = w.fileCache.Get(fileID)
				if err != nil {
					return err
				}
				filePath := gopath.Join(tmpSourceDir, fileNameWithPath)
				dir, _ := filepath.Split(filePath)
				if dir != "" {
					_ = os.MkdirAll(dir, 0755)
				}

				CopyFile(path, filePath)
				unlock()
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := 0
			var errStr *string = nil

			for _, cmdWithArgs := range jobSpec.Cmds {
				renderedCmd, err := cmdWithArgs.Render(build.JobContext{
					SourceDir: tmpSourceDir,
					OutputDir: ".",
					Deps:      nil, // TODO
				})
				if err != nil {
					// TODO
				}
				spew.Dump(renderedCmd)

				if len(renderedCmd.Exec) > 0 {
					cmd := exec.Command(renderedCmd.Exec[0], renderedCmd.Exec[1:]...)
					cmd.Stdout = &stdout
					cmd.Stderr = &stderr
					err := cmd.Run()
					if err == nil {
						continue
					}
					if exitError, ok := err.(*exec.ExitError); ok {
						exitCode = exitError.ExitCode()
					}
					str := err.Error()
					errStr = &str
					break
				}
				if renderedCmd.CatOutput != "" {
					err := ioutil.WriteFile(renderedCmd.CatOutput, []byte(renderedCmd.CatTemplate), 0666)
					if err == nil {
						continue
					}
					str := err.Error()
					errStr = &str
					break
				}
			}
			jobResult := api.JobResult{
				ID:       jobSpec.ID,
				Stdout:   stdout.Bytes(),
				Stderr:   stderr.Bytes(),
				ExitCode: exitCode,
				Error:    errStr,
			}
			finishedJob = append(finishedJob, jobResult)
		}
	}
}
