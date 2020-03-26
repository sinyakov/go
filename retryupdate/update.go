// +build !solution

package retryupdate

import (
	"errors"

	"github.com/gofrs/uuid"
	"gitlab.com/slon/shad-go/retryupdate/kvapi"
)

func UpdateValue(c kvapi.Client, key string, updateFn func(oldValue *string) (newValue string, err error)) error {
	var oldValue *string
	var err error
	var getResp *kvapi.GetResponse

	for {
		for {
			oldValue = nil
			getResp, err = c.Get(&kvapi.GetRequest{Key: key})
			if err == nil {
				oldValue = &getResp.Value
				break
			}

			upwrappedErr := errors.Unwrap(err)

			if upwrappedErr == kvapi.ErrKeyNotFound {
				break
			}

			if _, ok := upwrappedErr.(*kvapi.AuthError); ok {
				return err
			}
		}

		newValue, err := updateFn(oldValue)
		if err != nil {
			return err
		}

		oldVersion := uuid.UUID{}

		if getResp != nil {
			oldVersion = getResp.Version
		}

		newVersion := uuid.Must(uuid.NewV4())

		for {
			_, err = c.Set(&kvapi.SetRequest{
				Key:        key,
				Value:      newValue,
				OldVersion: oldVersion,
				NewVersion: newVersion,
			})

			if err == nil {
				return nil
			}

			upwrappedErr := errors.Unwrap(err)

			if _, ok := upwrappedErr.(*kvapi.AuthError); ok {
				return err
			}

			if upwrappedErr == kvapi.ErrKeyNotFound {
				oldVersion = uuid.UUID{}
				newValue, _ = updateFn(nil)
				continue
			}

			if conflictErr, ok := upwrappedErr.(*kvapi.ConflictError); ok {
				if newVersion != conflictErr.ExpectedVersion {
					break
				}
				return nil
			}

			if _, ok := err.(*kvapi.APIError); ok {
				continue
			}
		}
	}
}
