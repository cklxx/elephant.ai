package http

import "container/list"

type stringLRUEntry struct {
	key   string
	value string
}

type stringLRU struct {
	capacity int
	items    map[string]*list.Element
	order    *list.List
}

func newStringLRU(capacity int) *stringLRU {
	if capacity <= 0 {
		return &stringLRU{capacity: 0}
	}
	return &stringLRU{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

func (c *stringLRU) Len() int {
	if c == nil || c.capacity <= 0 {
		return 0
	}
	return len(c.items)
}

func (c *stringLRU) Get(key string) (string, bool) {
	if c == nil || c.capacity <= 0 {
		return "", false
	}
	entry, ok := c.items[key]
	if !ok {
		return "", false
	}
	c.order.MoveToFront(entry)
	val, _ := entry.Value.(stringLRUEntry)
	return val.value, true
}

func (c *stringLRU) Set(key, value string) {
	if c == nil || c.capacity <= 0 {
		return
	}
	if entry, ok := c.items[key]; ok {
		entry.Value = stringLRUEntry{key: key, value: value}
		c.order.MoveToFront(entry)
		return
	}
	element := c.order.PushFront(stringLRUEntry{key: key, value: value})
	c.items[key] = element
	for len(c.items) > c.capacity {
		c.evictOldest()
	}
}

func (c *stringLRU) Delete(key string) {
	if c == nil || c.capacity <= 0 {
		return
	}
	if entry, ok := c.items[key]; ok {
		c.order.Remove(entry)
		delete(c.items, key)
	}
}

func (c *stringLRU) evictOldest() {
	if c == nil || c.capacity <= 0 || c.order == nil {
		return
	}
	oldest := c.order.Back()
	if oldest == nil {
		return
	}
	entry, ok := oldest.Value.(stringLRUEntry)
	if ok {
		delete(c.items, entry.key)
	}
	c.order.Remove(oldest)
}

type runSeqEntry struct {
	key string
	seq uint64
}

type runSeqLRU struct {
	capacity int
	items    map[string]*list.Element
	order    *list.List
}

func newRunSeqLRU(capacity int) *runSeqLRU {
	if capacity <= 0 {
		return &runSeqLRU{capacity: 0}
	}
	return &runSeqLRU{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

func (c *runSeqLRU) Get(key string) (uint64, bool) {
	if c == nil || c.capacity <= 0 {
		return 0, false
	}
	entry, ok := c.items[key]
	if !ok {
		return 0, false
	}
	c.order.MoveToFront(entry)
	val, _ := entry.Value.(runSeqEntry)
	return val.seq, true
}

func (c *runSeqLRU) Set(key string, seq uint64) {
	if c == nil || c.capacity <= 0 {
		return
	}
	if entry, ok := c.items[key]; ok {
		entry.Value = runSeqEntry{key: key, seq: seq}
		c.order.MoveToFront(entry)
		return
	}
	element := c.order.PushFront(runSeqEntry{key: key, seq: seq})
	c.items[key] = element
	for len(c.items) > c.capacity {
		c.evictOldest()
	}
}

func (c *runSeqLRU) evictOldest() {
	if c == nil || c.capacity <= 0 || c.order == nil {
		return
	}
	oldest := c.order.Back()
	if oldest == nil {
		return
	}
	entry, ok := oldest.Value.(runSeqEntry)
	if ok {
		delete(c.items, entry.key)
	}
	c.order.Remove(oldest)
}
