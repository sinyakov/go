// +build !solution

package artifact

import (
	"errors"
	"os"
	gopath "path"
	"sync"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

var (
	ErrNotFound    = errors.New("artifact not found")
	ErrExists      = errors.New("artifact exists")
	ErrWriteLocked = errors.New("artifact is locked for write")
	ErrReadLocked  = errors.New("artifact is locked for read")
)

type ArtifactCache struct {
	// mu          *sync.RWMutex
	readLocked  bool
	writeLocked bool
}

type Cache struct {
	rootDir        string
	artifactsMutex *sync.Mutex
	artifacts      map[build.ID]*ArtifactCache
}

func NewCache(root string) (*Cache, error) {
	return &Cache{
		rootDir:        root,
		artifactsMutex: &sync.Mutex{},
		artifacts:      make(map[build.ID]*ArtifactCache),
	}, nil
}

func (c *Cache) Range(artifactFn func(artifact build.ID) error) error {
	c.artifactsMutex.Lock()
	defer c.artifactsMutex.Unlock()

	for artifact := range c.artifacts {
		err := artifactFn(artifact)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) Remove(artifact build.ID) error {
	c.artifactsMutex.Lock()
	defer c.artifactsMutex.Unlock()

	// fmt.Println(artifactCache)
	artifactCache, exists := c.artifacts[artifact]
	if !exists {
		return ErrNotFound
	}

	if artifactCache.writeLocked {
		return ErrWriteLocked
	}

	if artifactCache.readLocked {
		return ErrReadLocked
	}

	delete(c.artifacts, artifact)

	return nil
}

func (c *Cache) Create(artifact build.ID) (path string, commit, abort func() error, err error) {
	c.artifactsMutex.Lock()
	defer c.artifactsMutex.Unlock()

	currentArtifactCache, exists := c.artifacts[artifact]
	if exists {
		if currentArtifactCache.writeLocked {
			return "", nil, nil, ErrWriteLocked
		}
		return "", nil, nil, ErrExists
	}

	path = gopath.Join(c.rootDir, artifact.String())
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return "", nil, nil, err
	}

	artifactCache := &ArtifactCache{
		// mu:          &sync.RWMutex{},
		readLocked:  false,
		writeLocked: true,
	}
	c.artifacts[artifact] = artifactCache

	commit = func() error {
		c.artifactsMutex.Lock()
		defer c.artifactsMutex.Unlock()

		_, exists := c.artifacts[artifact]
		if !exists {
			return ErrNotFound
		}

		artifactCache.writeLocked = false
		return nil
	}

	abort = func() error {
		c.artifactsMutex.Lock()
		defer c.artifactsMutex.Unlock()

		err := os.RemoveAll(path)
		if err != nil {
			return err
		}

		delete(c.artifacts, artifact)

		return nil
	}

	return c.rootDir, commit, abort, nil

}

func (c *Cache) Get(artifact build.ID) (path string, unlock func(), err error) {
	c.artifactsMutex.Lock()
	defer c.artifactsMutex.Unlock()

	artifactCache, exists := c.artifacts[artifact]
	if !exists {
		return "", nil, ErrNotFound
	}

	if artifactCache.writeLocked {
		return "", nil, ErrWriteLocked
	}

	if artifactCache.readLocked {
		return "", nil, ErrReadLocked
	}

	artifactCache.readLocked = true

	path = gopath.Join(c.rootDir, artifact.String())
	unlock = func() {
		c.artifactsMutex.Lock()
		defer c.artifactsMutex.Unlock()

		artifactCache.readLocked = false
	}

	return c.rootDir, unlock, nil
}
