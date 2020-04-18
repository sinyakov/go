// +build !solution

package filecache

import (
	"context"
	"io"
	"net/http"
	"os"

	"go.uber.org/zap"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

type Client struct {
	logger   *zap.Logger
	endpoint string
}

func NewClient(l *zap.Logger, endpoint string) *Client {
	return &Client{
		logger:   l,
		endpoint: endpoint,
	}
}

func (c *Client) Upload(ctx context.Context, id build.ID, localPath string) error {
	f, err := os.OpenFile(localPath, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	url := c.endpoint + "/file?id=" + id.String()
	httpClient := &http.Client{}

	req, err := http.NewRequest(http.MethodPut, url, f)
	if err != nil {
		c.logger.Error("Filecache Create request error")
		return err
	}

	_, err = httpClient.Do(req)
	if err != nil {
		c.logger.Error("Filecache Upload file error")
		return err
	}

	return nil
}

func (c *Client) Download(ctx context.Context, localCache *Cache, id build.ID) error {
	resp, err := http.Get(c.endpoint + "/file?id=" + id.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	w, abort, err := localCache.Write(id)
	if err != nil {
		return err
	}

	_, _ = io.Copy(w, resp.Body)
	if err != nil {
		abort()
		return err
	}

	return nil
}
