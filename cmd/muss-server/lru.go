package main

// This package provides a simple LRU cache. It is based on the
// LRU implementation in groupcache:
// https://github.com/golang/groupcache/tree/master/lru
import (
	"container/list"
	"errors"
	"sync"
)

// EvictCallback is used to get a callback when a cache entry is evicted
type EvictCallback func(key int, value interface{})

// LRU implements a non-thread safe fixed size LRU cache
type LRU struct {
	size      int
	used      int
	evictList *list.List
	items     map[int]*list.Element
	onEvict   EvictCallback
	lock      sync.Mutex
}

// entry is used to hold a value in the evictList
type entry struct {
	key   int
	value interface{}
	size  int
}

// NewLRU constructs an LRU of the given size
func NewLRU(size int, onEvict EvictCallback) (*LRU, error) {
	if size <= 0 {
		return nil, errors.New("Must provide a positive size")
	}
	c := &LRU{
		size:      size,
		evictList: list.New(),
		items:     make(map[int]*list.Element),
		onEvict:   onEvict,
	}
	return c, nil
}

// Purge is used to completely clear the cache
func (c *LRU) Purge() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for k, v := range c.items {
		if c.onEvict != nil {
			c.onEvict(k, v.Value.(*entry).value)
		}
		delete(c.items, k)
	}
	c.evictList.Init()
	c.used = 0
}

// Add adds a value to the cache.  Returns true if an eviction occured.
func (c *LRU) Add(key int, value interface{}) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	size := 1
	// Check for existing item
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		originSize := ent.Value.(*entry).size
		ent.Value.(*entry).value = value
		ent.Value.(*entry).size = size
		delta := size - originSize
		c.used += delta
		evict := false
		if delta > 0 {
			length := c.evictList.Len()
			for i := 0; i < length; i++ {
				if c.used > c.size {
					c.removeOldest()
					evict = true
				} else {
					break
				}
			}
		}
		return evict
	}

	// Add new item
	ent := &entry{key, value, size}
	entry := c.evictList.PushFront(ent)
	c.used += size
	c.items[key] = entry

	evict := false
	length := c.evictList.Len()
	for i := 0; i < length; i++ {
		// Verify size not exceeded
		if c.used > c.size {
			c.removeOldest()
			evict = true
		} else {
			break
		}
	}
	return evict
}

// Get looks up a key's value from the cache.
func (c *LRU) Get(key int) (value interface{}, ok bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		return ent.Value.(*entry).value, true
	}
	return nil, false
}

// Check if a key is in the cache, without updating the recent-ness
// or deleting it for being stale.
func (c *LRU) Contains(key int) (ok bool) {
	_, ok = c.items[key]
	return ok
}

// Returns the key value (or undefined if not found) without updating
// the "recently used"-ness of the key.
func (c *LRU) Peek(key int) (value interface{}, size int, ok bool) {
	if ent, ok := c.items[key]; ok {
		return ent.Value.(*entry).value, ent.Value.(*entry).size, true
	}
	return nil, 0, ok
}

// Remove removes the provided key from the cache, returning if the
// key was contained.
func (c *LRU) Remove(key int) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	if ent, ok := c.items[key]; ok {
		c.removeElement(ent)
		return true
	}
	return false
}

// RemoveOldest removes the oldest item from the cache.
func (c *LRU) RemoveOldest() (int, interface{}, int, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
		kv := ent.Value.(*entry)
		return kv.key, kv.value, kv.size, true
	}
	return 0, nil, 0, false
}

// GetOldest returns the oldest entry
func (c *LRU) GetOldest() (int, interface{}, bool) {
	ent := c.evictList.Back()
	if ent != nil {
		kv := ent.Value.(*entry)
		return kv.key, kv.value, true
	}
	return 0, nil, false
}

// Keys returns a slice of the keys in the cache, from oldest to newest.
func (c *LRU) Keys() []int {
	keys := make([]int, len(c.items))
	i := 0
	for ent := c.evictList.Back(); ent != nil; ent = ent.Prev() {
		keys[i] = ent.Value.(*entry).key
		i++
	}
	return keys
}

// Len returns the number of items in the cache.
func (c *LRU) Len() int {
	return c.evictList.Len()
}

func (c *LRU) Size() int {
	return c.used
}

// removeOldest removes the oldest item from the cache.
func (c *LRU) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

// removeElement is used to remove a given list element from the cache
func (c *LRU) removeElement(e *list.Element) {
	c.evictList.Remove(e)
	kv := e.Value.(*entry)
	c.used -= kv.size
	delete(c.items, kv.key)
	if c.onEvict != nil {
		c.onEvict(kv.key, kv.value)
	}
}
