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
