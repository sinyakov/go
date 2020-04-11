// +build !solution

package batcher

import (
	"sync"
	"time"

	"gitlab.com/slon/shad-go/batcher/slow"
)

type Batcher struct {
	sync.RWMutex
	slowValue         *slow.Value
	lastValue         interface{}
	isLoading         bool
	waitLoadCh        chan struct{}
	lastLoadStarted   time.Time
	lastLoadCompleted time.Time
}

func NewBatcher(v *slow.Value) *Batcher {
	return &Batcher{
		slowValue:  v,
		waitLoadCh: make(chan struct{}),
	}
}

func (b *Batcher) Load() interface{} {
	if b.lastLoadStarted.UnixNano() > b.lastLoadCompleted.UnixNano() && b.isLoading {
		<-b.waitLoadCh
	}

	b.Lock()

	if b.isLoading {
		b.Unlock()
		<-b.waitLoadCh
		return b.lastValue
	}

	if !b.isLoading {
		b.isLoading = true
		b.lastLoadStarted = time.Now()
		b.Unlock()

		b.lastValue = b.slowValue.Load()

		b.Lock()
		b.lastLoadCompleted = time.Now()

		close(b.waitLoadCh)
		b.waitLoadCh = make(chan struct{})
		b.isLoading = false
		b.Unlock()
		return b.lastValue
	}

	return b.lastValue
}
