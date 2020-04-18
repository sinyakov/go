// +build !solution

package filecache

import (
	"errors"
	"io"
	"os"
	gopath "path"
	"path/filepath"
	"sync"

	"gitlab.com/slon/shad-go/distbuild/pkg/build"
)

var (
	ErrNotFound    = errors.New("file not found")
	ErrExists      = errors.New("file exists")
	ErrWriteLocked = errors.New("file is locked for write")
	ErrReadLocked  = errors.New("file is locked for read")
)

type FileCache struct {
	readLocked  bool
	writeLocked bool
}

type Cache struct {
	rootDir    string
	filesMutex *sync.Mutex
	files      map[build.ID]*FileCache
}

func New(rootDir string) (*Cache, error) {
	return &Cache{
		rootDir:    rootDir,
		filesMutex: &sync.Mutex{},
		files:      make(map[build.ID]*FileCache),
	}, nil
}

func (c *Cache) Range(fileFn func(file build.ID) error) error {
	c.filesMutex.Lock()
	defer c.filesMutex.Unlock()

	for file := range c.files {
		err := fileFn(file)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Cache) Remove(file build.ID) error {
	c.filesMutex.Lock()
	defer c.filesMutex.Unlock()

	fileCache, exists := c.files[file]
	if !exists {
		return ErrNotFound
	}

	if fileCache.writeLocked {
		return ErrWriteLocked
	}

	if fileCache.readLocked {
		return ErrReadLocked
	}

	delete(c.files, file)

	return nil
}

func (c *Cache) Write(file build.ID) (w io.WriteCloser, abort func() error, err error) {
	c.filesMutex.Lock()
	defer c.filesMutex.Unlock()

	currentFileCache, exists := c.files[file]
	if exists {
		if currentFileCache.writeLocked {
			return nil, nil, ErrWriteLocked
		}
		return nil, nil, ErrExists
	}

	filePath := gopath.Join(c.rootDir, file.Path())
	dir, _ := filepath.Split(filePath)
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)

	if err != nil {
		return nil, nil, err
	}

	fileCache := &FileCache{
		readLocked:  false,
		writeLocked: false,
	}
	c.files[file] = fileCache

	abort = func() error {
		c.filesMutex.Lock()
		defer c.filesMutex.Unlock()

		err := os.Remove(filePath)
		if err != nil {
			return err
		}

		delete(c.files, file)

		return nil
	}

	return f, abort, nil

}

func (c *Cache) Get(file build.ID) (path string, unlock func(), err error) {
	c.filesMutex.Lock()
	defer c.filesMutex.Unlock()

	filePath := gopath.Join(c.rootDir, file.Path())

	_, err = os.Stat(filePath)

	if err != nil {
		return "", nil, ErrNotFound
	}

	fileCache, exists := c.files[file]
	if exists {
		if fileCache.writeLocked {
			return "", nil, ErrWriteLocked
		}

		if fileCache.readLocked {
			return "", nil, ErrReadLocked
		}

	} else {
		c.files[file] = &FileCache{
			readLocked:  true,
			writeLocked: false,
		}
	}

	unlock = func() {
		c.filesMutex.Lock()
		defer c.filesMutex.Unlock()

		c.files[file].readLocked = false
	}

	return filePath, unlock, nil
}
