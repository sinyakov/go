// +build !solution

package keylock

import (
	"sync"
)

type KeyLock struct {
	lockedKeys    map[string]bool
	mux           *sync.Mutex
	unlockedEvent chan struct{}
}

func New() *KeyLock {
	return &KeyLock{
		lockedKeys:    make(map[string]bool),
		mux:           &sync.Mutex{},
		unlockedEvent: make(chan struct{}, 100000),
	}
}

func (l *KeyLock) HasLockedKeys(keys []string) bool {
	for _, key := range keys {
		_, exists := l.lockedKeys[key]

		if exists {
			return true
		}
	}
	return false
}

func (l *KeyLock) lockKeys(keys []string) {
	for _, key := range keys {
		l.lockedKeys[key] = true
	}
}

func (l *KeyLock) unlockKeys(keys []string) {
	l.mux.Lock()
	defer l.mux.Unlock()

	for _, key := range keys {
		delete(l.lockedKeys, key)
	}

	if len(l.unlockedEvent) < 3 {
		l.unlockedEvent <- struct{}{}
	}
}

func (l *KeyLock) LockKeys(keys []string, cancel <-chan struct{}) (canceled bool, unlock func()) {
	l.mux.Lock()
	defer l.mux.Unlock()

	if !l.HasLockedKeys(keys) {
		l.lockKeys(keys)

		keysCopy := make([]string, len(keys))
		copy(keysCopy, keys)

		return false, func() {
			l.unlockKeys(keysCopy)
		}
	}

	for {
		select {
		case <-l.unlockedEvent:
			if !l.HasLockedKeys(keys) {
				l.lockKeys(keys)

				keysCopy := make([]string, len(keys))
				copy(keysCopy, keys)

				return false, func() {
					l.unlockKeys(keysCopy)
				}
			}
		default:
			if cancel == nil {
				return false, func() {}
			}
			return true, nil
		}
	}
}
