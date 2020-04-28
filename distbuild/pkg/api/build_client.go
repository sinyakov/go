// +build !solution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

type statusReader struct {
	body io.ReadCloser
}

func NewStatusReader(body io.ReadCloser) *statusReader {
	return &statusReader{
		body: body,
	}
}

func (sr *statusReader) Close() error {
	return sr.body.Close()
}

func (sr *statusReader) Next() (*StatusUpdate, error) {
	var resp StatusUpdate
	decoder := json.NewDecoder(sr.body)
	err := decoder.Decode(&resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

type BuildClient struct {
	logger   *zap.Logger
	endpoint string
}

func NewBuildClient(l *zap.Logger, endpoint string) *BuildClient {
	return &BuildClient{
		logger:   l,
		endpoint: endpoint,
	}
}

func (c *BuildClient) StartBuild(ctx context.Context, request *BuildRequest) (*BuildStarted, StatusReader, error) {
	c.logger.Info("pkg/api/build_client.go StartBuild start")

	httpClient := &http.Client{}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(request)

	// fmt.Println(">>>>1", b.String())
	req, err := http.NewRequest(http.MethodPost, c.endpoint+"/build", b)
	if err != nil {
		c.logger.Error("pkg/api/build_client.go StartBuild request", zap.Error(err))
		return nil, nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		c.logger.Error("pkg/api/build_client.go StartBuild do request", zap.Error(err))
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		msg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		resp.Body.Close()
		return nil, nil, errors.New(string(msg))
	}

	var buildStarted BuildStarted
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&buildStarted)

	return &buildStarted, NewStatusReader(resp.Body), nil
}

func (c *BuildClient) SignalBuild(ctx context.Context, buildID build.ID, signal *SignalRequest) (*SignalResponse, error) {
	c.logger.Info("pkg/api/build_client.go SignalBuild start", zap.String("buildId", buildID.String()))

	url := c.endpoint + "/signal?build_id=" + buildID.String()
	httpClient := &http.Client{}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(signal)

	req, err := http.NewRequest(http.MethodPost, url, b)
	if err != nil {
		c.logger.Error("pkg/api/build_client.go SignalBuild request", zap.Error(err))
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		c.logger.Error("pkg/api/build_client.go SignalBuild do request", zap.Error(err))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		msg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(msg))
	}

	var signalResponse SignalResponse
	err = json.NewDecoder(resp.Body).Decode(&signalResponse)
	if err != nil {
		c.logger.Error("pkg/api/build_client.go SignalBuild body parse", zap.Error(err))
		return nil, err
	}
	return &signalResponse, nil
}
