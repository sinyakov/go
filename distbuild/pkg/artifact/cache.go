// +build !solution

package artifact

import (
	"errors"
	"fmt"
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
	// TODO: поменять на проход по файлам
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

	filePath := gopath.Join(c.rootDir, artifact.Path())

	_, err := os.Stat(filePath)

	if err != nil {
		return ErrNotFound
	}

	artifactCache, exists := c.artifacts[artifact]
	if exists {
		if artifactCache.writeLocked {
			return ErrWriteLocked
		}

		if artifactCache.readLocked {
			return ErrReadLocked
		}
	}

	err = os.RemoveAll(filePath)
	if err != nil {
		return err
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

	filePath := gopath.Join(c.rootDir, artifact.Path())
	_ = os.MkdirAll(filePath, 0755)

	fmt.Println("Create", filePath)

	// path = gopath.Join(c.rootDir, artifact.Path())
	// err = os.MkdirAll(path, 0755)
	// if err != nil {
	// 	return "", nil, nil, err
	// }

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

		err := os.RemoveAll(filePath)
		if err != nil {
			return err
		}

		delete(c.artifacts, artifact)

		return nil
	}

	return filePath, commit, abort, nil

}

func (c *Cache) Get(artifact build.ID) (path string, unlock func(), err error) {
	c.artifactsMutex.Lock()
	defer c.artifactsMutex.Unlock()

	filePath := gopath.Join(c.rootDir, artifact.Path())
	fmt.Println("Get   ", filePath)

	_, err = os.Stat(filePath)

	if err != nil {
		return "", nil, ErrNotFound
	}

	artifactCache, exists := c.artifacts[artifact]
	if exists {
		if artifactCache.writeLocked {
			return "", nil, ErrWriteLocked
		}

		if artifactCache.readLocked {
			return "", nil, ErrReadLocked
		}
	} else {
		c.artifacts[artifact] = &ArtifactCache{}
	}
	c.artifacts[artifact].readLocked = true

	unlock = func() {
		c.artifactsMutex.Lock()
		defer c.artifactsMutex.Unlock()

		if _, exists := c.artifacts[artifact]; exists {
			c.artifacts[artifact].readLocked = false
		}
	}

	return filePath, unlock, nil
}
