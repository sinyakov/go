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
	"strings"

	// "time"

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
	mux                 *http.ServeMux
}

func New(
	workerID api.WorkerID,
	coordinatorEndpoint string,
	log *zap.Logger,
	fileCache *filecache.Cache,
	artifacts *artifact.Cache,
) *Worker {
	mux := http.NewServeMux()

	artifactsHandler := artifact.NewHandler(log, artifacts)
	artifactsHandler.Register(mux)

	return &Worker{
		workerID:            workerID,
		coordinatorEndpoint: coordinatorEndpoint,
		logger:              log,
		fileCache:           fileCache,
		artifacts:           artifacts,
		filecacheClient:     filecache.NewClient(log, coordinatorEndpoint),
		mux:                 mux,
	}
}

func (w *Worker) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w.logger.Info("pkg/worker/worker.go ServeHTTP", zap.String("Request URI", r.RequestURI))
	w.mux.ServeHTTP(rw, r)
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

		finishedJob = []api.JobResult{}
		// fmt.Println("resp.JobsToRun", len(resp.JobsToRun))
		for _, jobSpec := range resp.JobsToRun {
			outputDir := gopath.Join(os.TempDir(), jobSpec.ID.String())
			tmpSourceDir := gopath.Join(os.TempDir(), jobSpec.ID.String()+".tmp")

			_ = os.MkdirAll(outputDir, 0755)
			_ = os.MkdirAll(tmpSourceDir, 0755)

			depsMap := map[build.ID]string{}
			for artifactID, workerID := range jobSpec.Artifacts {
				fmt.Println(">>", string(workerID))
				if workerID != w.workerID {
					for {
						err := artifact.Download(ctx, string(workerID), w.artifacts, artifactID)
						if err == nil {
							break
						}
						if strings.Contains(err.Error(), "locked") {
							// w.logger.Error("pkg/worker/worker.go Download waiting for unlock due", zap.String("workerID", string(workerID)), zap.Error(err))
							continue
						}
						if strings.Contains(err.Error(), "artifact exists") {
							// time.Sleep(time.Millisecond * 500) // TODO: BUG: удалить
							// w.logger.Error("pkg/worker/worker.go Download", zap.String("workerID", string(workerID)), zap.Error(err))
							break
						}
						if err != nil {
							w.logger.Error("pkg/worker/worker.go Download", zap.String("workerID", string(workerID)), zap.Error(err))
							// continue
							return err
						}
					}
				}

				for {
					artifactPath, unlockFn, err := w.artifacts.Get(artifactID)
					if err == nil {
						depsMap[artifactID] = artifactPath
						if unlockFn != nil {
							unlockFn()
						}
						break
					}
					if strings.Contains(err.Error(), "locked") {
						// w.logger.Error("pkg/worker/worker.go Download", zap.String("workerID", string(workerID)), zap.Error(err))
						// unlockFn()
						continue
					}
					return err
				}
			}
			// fmt.Printf("\nWorker.run workerID: %s deps map: %+v\n", w.workerID, depsMap)

			for fileID, fileNameWithPath := range jobSpec.SourceFiles {
				_, unlock, err := w.fileCache.Get(fileID)
				if err == nil {
					unlock()
					continue
				}
				err = w.filecacheClient.Download(ctx, w.fileCache, fileID)
				if err != nil {
					return err
				}
				path, unlock, err := w.fileCache.Get(fileID)
				if err != nil {
					return err
				}
				filePath := gopath.Join(tmpSourceDir, fileNameWithPath)
				dir, _ := filepath.Split(filePath)
				if dir != "" {
					_ = os.MkdirAll(dir, 0755)
				}

				errCopyFile := CopyFile(path, filePath)
				if err != nil {
					return errCopyFile
				}
				unlock()
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := 0
			var errStr *string = nil

			for _, cmdWithArgs := range jobSpec.Cmds {
				renderedCmd, err := cmdWithArgs.Render(build.JobContext{
					SourceDir: tmpSourceDir,
					OutputDir: outputDir,
					Deps:      depsMap,
				})
				if err != nil {
					return err
				}
				// spew.Dump(renderedCmd)

				if len(renderedCmd.Exec) > 0 {
					fmt.Printf("    WorkerID: %s, Exec: %s\n", w.workerID, renderedCmd.Exec[0])
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
					fmt.Printf("    WorkerID: %s, CatOutput: %s\n", w.workerID, renderedCmd.CatOutput)
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
			artifactsDir, commitFn, abortFn, err := w.artifacts.Create(jobSpec.ID)
			if err != nil {
				// TODO в finishedJob записать
				return err
			}

			err = CopyDirectory(outputDir, artifactsDir)
			if err != nil {
				errAbort := abortFn()
				if errAbort != nil {
					return errAbort
				}
				continue
			}

			errCommit := commitFn()
			if errCommit != nil {
				return errCommit
			}
			finishedJob = append(finishedJob, jobResult)
		}
	}
}
