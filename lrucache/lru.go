// +build !solution

package lrucache

import (
	"container/list"
)

type cache struct {
	elems map[int]*list.Element
	seq   *list.List
	cap   int
}

type cacheElement struct {
	key   int
	value int
}

func New(cap int) Cache {
	return &cache{
		elems: make(map[int]*list.Element),
		seq:   list.New(),
		cap:   cap,
	}
}

func (c *cache) Clear() {
	for k := range c.elems {
		delete(c.elems, k)
	}
	c.seq.Init()
}

func (c *cache) Get(key int) (int, bool) {
	if elem, ok := c.elems[key]; ok {
		c.seq.MoveToFront(elem)
		return elem.Value.(*cacheElement).value, true
	}

	return 0, false
}

func (c *cache) Set(key, value int) {
	if c.cap == 0 {
		return
	}

	if elem, ok := c.elems[key]; ok {
		c.seq.MoveToFront(elem)
		elem.Value.(*cacheElement).value = value
		return
	}

	seqItem := c.seq.PushFront(&cacheElement{
		key:   key,
		value: value,
	})

	c.elems[key] = seqItem

	if c.seq.Len() > c.cap {
		c.seq.Remove(c.seq.Back())
	}
}

func (c *cache) Range(f func(key, value int) bool) {
	for e := c.seq.Back(); e != nil; e = e.Prev() {
		elem := e.Value.(*cacheElement)
		if !f(elem.key, elem.value) {
			break
		}
	}
}
