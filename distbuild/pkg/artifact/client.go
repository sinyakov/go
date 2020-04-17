// +build !solution

package artifact

import (
	"context"
	"net/http"

	"gitlab.com/slon/shad-go/distbuild/pkg/tarstream"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

// Download artifact from remote cache into local cache.
func Download(ctx context.Context, endpoint string, c *Cache, artifactID build.ID) error {
	resp, err := http.Get(endpoint + "/artifact?id=" + artifactID.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	path, commit, abort, err := c.Create(artifactID)
	if err != nil {
		return err
	}

	err = tarstream.Receive(path, resp.Body)
	if err != nil {
		abort()
		return err
	}

	commit()
	return nil
}
