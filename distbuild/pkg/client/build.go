// +build !solution

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	gopath "path"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/filecache"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

type Client struct {
	logger          *zap.Logger
	apiEndpoint     string
	sourceDir       string
	filecacheClient *filecache.Client
	idx             int
}

func NewClient(
	l *zap.Logger,
	apiEndpoint string,
	sourceDir string,
) *Client {
	return &Client{
		logger:          l,
		apiEndpoint:     apiEndpoint,
		sourceDir:       sourceDir,
		filecacheClient: filecache.NewClient(l, apiEndpoint),
		idx:             0,
	}
}

type BuildListener interface {
	OnJobStdout(jobID build.ID, stdout []byte) error
	OnJobStderr(jobID build.ID, stderr []byte) error

	OnJobFinished(jobID build.ID) error
	OnJobFailed(jobID build.ID, code int, error string) error
}

func (c *Client) Build(ctx context.Context, graph build.Graph, lsn BuildListener) error {
	buildClient := api.NewBuildClient(c.logger, c.apiEndpoint)
	buildStarted, statusReader, err := buildClient.StartBuild(ctx, &api.BuildRequest{Graph: graph})
	if err != nil {
		c.logger.Error("pkg/client/build.go Build", zap.Error(err))
		return err
	}
	defer statusReader.Close()

	if len(buildStarted.MissingFiles) > 0 {
		for _, fileID := range buildStarted.MissingFiles {
			err := c.filecacheClient.Upload(ctx, fileID, gopath.Join(c.sourceDir, graph.SourceFiles[fileID]))
			if err != nil {
				c.logger.Error("pkg/client/build.go Build Upload", zap.Error(err))
				return err
			}
		}
	}

	// TODO: заливка отсутствующих файлов

	idx := c.idx
	c.idx++

	for {
		statusUpdate, err := statusReader.Next()
		fmt.Printf("DDD: [%d] statusReader.Next(): пришел ответ %+v, %v\n", idx, statusUpdate, err)

		if statusUpdate != nil && statusUpdate.JobFinished != nil {
			fmt.Printf("DDD: [%d] statusReader.Next() задача выполнена %s\n\n", idx, statusUpdate.JobFinished.ID)
			lsn.OnJobStdout(statusUpdate.JobFinished.ID, statusUpdate.JobFinished.Stdout) // ADDED
			lsn.OnJobStderr(statusUpdate.JobFinished.ID, statusUpdate.JobFinished.Stderr) // ADDED
			lsn.OnJobFinished(statusUpdate.JobFinished.ID)
			// return nil
			continue
		} else {
			fmt.Printf("DDD: [%d] statusReader.Next() задача НЕ выполнена\n\n", idx)
		}

		// TODO: HACK
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}
	}
}
