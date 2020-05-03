// +build !solution

package filecache

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
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

	resp, err := httpClient.Do(req)

	if err != nil {
		c.logger.Error("Filecache Upload file error")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New("UPLOAD " + string(b))
	}

	return nil
}

func (c *Client) Download(ctx context.Context, localCache *Cache, id build.ID) error {
	resp, err := http.Get(c.endpoint + "/file?id=" + id.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New("DOWNLOAD " + string(b))
	}

	w, abort, err := localCache.Write(id)
	if err != nil {
		return err
	}

	_, _ = io.Copy(w, resp.Body)
	if err != nil {
		errAbort := abort()
		if errAbort != nil {
			return errAbort
		}
		return err
	}

	return nil
}
