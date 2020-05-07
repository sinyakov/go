package worker

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"gitlab.com/slon/shad-go/distbuild/pkg/api"
	"gitlab.com/slon/shad-go/distbuild/pkg/build"
	"gitlab.com/slon/shad-go/distbuild/pkg/dist"
)

func LocateArtifact(ctx context.Context, endpoint string, artifactID build.ID) (api.WorkerID, bool, error) {
	resp, err := http.Get(endpoint + "/locate_artifact?id=" + artifactID.String())
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}

	var respData dist.LocateArtifactResponse
	err = json.Unmarshal(out, &respData)
	if err != nil {
		return "", false, err
	}

	return respData.WorkerID, respData.Exists, nil
}
