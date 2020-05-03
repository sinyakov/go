// +build !solution

package client

import (
	"context"
	"errors"
	"fmt"
	"io"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

type Client struct {
	logger      *zap.Logger
	apiEndpoint string
	sourceDir   string
}

func NewClient(
	l *zap.Logger,
	apiEndpoint string,
	sourceDir string,
) *Client {
	return &Client{
		logger:      l,
		apiEndpoint: apiEndpoint,
		sourceDir:   sourceDir,
	}
}

type BuildListener interface {
	OnJobStdout(jobID build.ID, stdout []byte) error
	OnJobStderr(jobID build.ID, stderr []byte) error

	OnJobFinished(jobID build.ID) error
	OnJobFailed(jobID build.ID, code int, error string) error
}

func (c *Client) Build(ctx context.Context, graph build.Graph, lsn BuildListener) error {
	c.logger.Info("pkg/client/build.go Build start")
	buildClient := api.NewBuildClient(c.logger, c.apiEndpoint)
	_, statusReader, err := buildClient.StartBuild(ctx, &api.BuildRequest{Graph: graph})
	if err != nil {
		c.logger.Error("pkg/client/build.go Build", zap.Error(err))
		return err
	}
	defer statusReader.Close()
	// TODO: заливка отсутствующих файлов
	fmt.Println("pkg/client/build.go buildStarted")
	// spew.Dump(buildStarted)

	// lsn.OnJobStdout(graph.Jobs[0].ID, []byte("OK\n"))
	// lsn.OnJobFinished(graph.Jobs[0].ID)
	// return nil

	for {
		// time.Sleep(time.Millisecond * 500)
		c.logger.Info("pkg/client/build.go statusReader 1")
		statusUpdate, err := statusReader.Next()
		c.logger.Info("pkg/client/build.go statusReader 2")

		if statusUpdate != nil && statusUpdate.JobFinished != nil {
			c.logger.Info("pkg/client/build.go statusReader 3")
			lsn.OnJobStdout(statusUpdate.JobFinished.ID, statusUpdate.JobFinished.Stdout) // ADDED
			lsn.OnJobStderr(statusUpdate.JobFinished.ID, statusUpdate.JobFinished.Stderr) // ADDED
			lsn.OnJobFinished(statusUpdate.JobFinished.ID)
			c.logger.Info("pkg/client/build.go statusReader 4, exited")
			return nil
		}

		// TODO: HACK
		if errors.Is(err, io.EOF) {
			c.logger.Info("pkg/client/build.go statusReader 5, exited")
			return nil
		}
		c.logger.Info("pkg/client/build.go statusReader 6")

		if err != nil {
			c.logger.Info("pkg/client/build.go statusReader 7, exited")
			return err
		}
		c.logger.Info("pkg/client/build.go statusReader 8, exited")
	}
}
