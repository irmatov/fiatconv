package cache

import (
	"bytes"
	"testing"
)

func TestCache(t *testing.T) {
	c := Load(bytes.NewBuffer(nil), 1000)
	if len(c.m) != 0 {
		t.Errorf("cache is not empty, has %v elems", len(c.m))
	}

	b := bytes.NewBuffer([]byte("garbage"))
	c = Load(b, 1000)
	if len(c.m) != 0 {
		t.Errorf("cache is not empty, has %v elems", len(c.m))
	}

	// set some values, check they are present.
	const nItems = 10
	for i := 0; i < nItems; i++ {
		c.Set(i, 2*i, int64(i))
	}
	for i := 0; i < nItems; i++ {
		if v, ok := c.Get(i); ok {
			if got := v.(int); got != 2*i {
				t.Errorf("value for key %v is %v, want %v", i, got, 2*i)
			}
		} else {
			t.Errorf("key %v is missing", i)
		}
	}
	b.Reset()
	c.Save(b)

	// half of the items should be dropped
	c = Load(b, int64(nItems/2))
	for i := 0; i < nItems/2; i++ {
		if _, ok := c.Get(i); ok {
			t.Errorf("key %v should be absent", i)
		}
	}
	for i := nItems/2 + 1; i < nItems; i++ {
		if v, ok := c.Get(i); ok {
			if got := v.(int); got != 2*i {
				t.Errorf("value for key %v is %v, want %v", i, got, 2*i)
			}
		} else {
			t.Errorf("key %v is missing", i)
		}
	}
}
