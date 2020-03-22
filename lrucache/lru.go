// +build !solution

package lrucache

import (
	"container/list"
	"time"
)

type cache struct {
	elements *list.List
	cap      int
}

type cacheElement struct {
	key            int
	value          int
	lastAccessedAt time.Time
}

func New(cap int) Cache {
	return &cache{
		elements: list.New(),
		cap:      cap,
	}
}

func (c *cache) Clear() {
	c.elements = list.New()
}

func (c *cache) Get(key int) (int, bool) {
	for e := c.elements.Front(); e != nil; e = e.Next() {
		elem := e.Value.(*cacheElement)
		if elem.key == key {
			elem.lastAccessedAt = time.Now()
			return elem.value, true
		}
	}
	return 0, false
}

func (c *cache) Set(key, value int) {
	if c.cap == 0 {
		return
	}

	if c.elements.Front() == nil {
		c.elements.PushBack(&cacheElement{key: key, value: value})
	}

	oldestElement := c.elements.Front()
	oldestCacheElement := oldestElement.Value.(*cacheElement)

	for e := c.elements.Front(); e != nil; e = e.Next() {
		elem := e.Value.(*cacheElement)
		if elem.key == key {
			c.elements.Remove(e)
			c.elements.PushBack(&cacheElement{key: key, value: value})
			return
		}
		if elem.lastAccessedAt.Before(oldestCacheElement.lastAccessedAt) {
			oldestCacheElement = elem
			oldestElement = e
		}
	}

	if c.elements.Len() >= c.cap {
		// oldestCacheElement.key = key
		// oldestCacheElement.value = value
		// oldestCacheElement.lastAccessedAt = time.Time{}
		c.elements.Remove(oldestElement)
	}

	c.elements.PushBack(&cacheElement{key: key, value: value})
}

func (c *cache) Range(f func(key, value int) bool) {
	for e := c.elements.Front(); e != nil; e = e.Next() {
		elem := e.Value.(*cacheElement)
		if !f(elem.key, elem.value) {
			break
		}
	}
}
