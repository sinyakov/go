// +build !solution

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
)

type HeartbeatClient struct {
	logger   *zap.Logger
	endpoint string
}

func NewHeartbeatClient(l *zap.Logger, endpoint string) *HeartbeatClient {
	return &HeartbeatClient{
		logger:   l,
		endpoint: endpoint,
	}
}

func (c *HeartbeatClient) Heartbeat(ctx context.Context, heartbeatReq *HeartbeatRequest) (*HeartbeatResponse, error) {
	c.logger.Info("pkg/api/heartbeat_client.go Heartbeat")

	httpClient := &http.Client{}
	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(heartbeatReq)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/heartbeat", b)
	if err != nil {
		c.logger.Error("pkg/api/heartbeat_client.go Heartbeat request", zap.Error(err))
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		c.logger.Error("pkg/api/heartbeat_client.go Heartbeat do request", zap.Error(err))
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		msg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(msg))
	}

	var heartbeatResponse HeartbeatResponse
	err = json.NewDecoder(resp.Body).Decode(&heartbeatResponse)
	if err != nil {
		c.logger.Error("pkg/api/heartbeat_client.go Heartbeat body parse", zap.Error(err))
		return nil, err
	}
	// c.logger.Info("pkg/api/heartbeat_client.go Heartbeat response")
	// spew.Dump(heartbeatResponse)
	return &heartbeatResponse, nil
}
