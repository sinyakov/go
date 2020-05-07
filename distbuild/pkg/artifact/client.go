// +build !solution

package artifact

import (
	"context"
	"fmt"
	"io/ioutil"
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

	if resp.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(resp.Body)

		return fmt.Errorf("artifact DOWNLOAD %s", string(b))
	}

	path, commit, abort, err := c.Create(artifactID)
	if err != nil {
		return err
	}

	err = tarstream.Receive(path, resp.Body)
	if err != nil {
		errAbort := abort()
		if errAbort != nil {
			return errAbort
		}
		return err
	}

	errCommit := commit()
	if errCommit != nil {
		return errCommit
	}

	return nil
}
