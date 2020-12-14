// Package cache implements simple GOB encoded key value cache.
//
// Cache is intended for use with unimportant data: load and save errors are
// ignored. Also, expired entries are dropped/ checked only at cache load time,
// so it is not suitable for long lived processes.
package cache

import (
	"encoding/gob"
	"io"
)

type item struct {
	Expires int64
	Value   interface{}
}

// Cache implements simple key/value cache based on file with GOB encoding and value lifetime.
type Cache struct {
	m map[interface{}]item
}

// Load loads cache from given reader, ignoring entries whose expire time is before cutoff.
// Any errors during loading are ignored and empty cache is returned in that case.
func Load(r io.Reader, cutoff int64) *Cache {
	c := &Cache{make(map[interface{}]item)}

	if err := gob.NewDecoder(r).Decode(&c.m); err != nil {
		return c
	}

	// purge expired items
	for k, v := range c.m {
		if v.Expires < cutoff {
			delete(c.m, k)
		}
	}
	return c
}

// Save saves cache to given writer, ignoring any errors.
func (c *Cache) Save(w io.Writer) error {
	return gob.NewEncoder(w).Encode(c.m)
}

// Set sets value for the given key.
func (c *Cache) Set(key, value interface{}, expires int64) {
	c.m[key] = item{expires, value}
}

// Get fetches a value associated with the given key. Expiration time is not checked.
func (c *Cache) Get(key interface{}) (interface{}, bool) {
	if item, ok := c.m[key]; ok {
		return item.Value, true
	}
	return nil, false
}
