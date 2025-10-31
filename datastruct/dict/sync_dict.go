package dict

import "sync"

type SyncDict struct {
	m sync.Map
}

// MakeSyncDict creates a new SyncDict instance
func MakeSyncDict() *SyncDict {
	return &SyncDict{}
}

// Get returns the value of the key
func (d *SyncDict) Get(key string) (val interface{}, exists bool) {
	if value, ok := d.m.Load(key); ok {
		return value, true
	}
	return nil, false
}

// Len returns the number of key-value pairs
func (d *SyncDict) Len() int {
	length := 0
	d.m.Range(func(key, value interface{}) bool {
		length++
		return true
	})
	return length
}

// Put puts a key-value pair into the dict
func (d *SyncDict) Put(key string, val interface{}) (result int) {
	_, exists := d.m.Load(key)
	d.m.Store(key, val)
	if exists {
		return 0
	}
	return 1
}

// PutIfAbsent puts a key-value pair into the dict if the key doesn't exist
func (d *SyncDict) PutIfAbsent(key string, val interface{}) (result int) {
	_, exists := d.m.Load(key)
	if exists {
		return 0
	}
	d.m.Store(key, val)
	return 1
}

// PutIfExists puts a key-value pair into the dict if the key exists
func (d *SyncDict) PutIfExists(key string, val interface{}) (result int) {
	_, exists := d.m.Load(key)
	if !exists {
		return 0
	}
	d.m.Store(key, val)
	return 1
}

// Remove removes a key-value pair from the dict
func (d *SyncDict) Remove(key string) (result int) {
	_, exists := d.m.Load(key)
	if !exists {
		return 0
	}
	d.m.Delete(key)
	return 1
}

// ForEach iterates over all key-value pairs
func (d *SyncDict) ForEach(consumer Consumer) {
	d.m.Range(func(key, value interface{}) bool {
		consumer(key.(string), value)
		return true
	})
}

// Keys returns all keys
func (d *SyncDict) Keys() []string {
	keys := make([]string, d.Len())
	d.m.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

// RandomKeys returns a slice of n random keys, Duplicate keys may be returned
func (d *SyncDict) RandomKeys(n int) []string {
	keys := make([]string, d.Len())
	for i := 0; i < n; i++ {
		// 伪随机，返回第一个就 return false 终止
		d.m.Range(func(key, value interface{}) bool {
			keys = append(keys, key.(string))
			return false
		})
	}
	return keys
}

func (d *SyncDict) RandomDistinctKeys(n int) []string {
	result := make([]string, d.Len())
	i := 0
	d.m.Range(func(key, value interface{}) bool {
		result[i] = key.(string)
		i++
		return i != n
	})
	return result
}

// Clear removes all key-value pairs
func (d *SyncDict) Clear() {
	*d = *MakeSyncDict() // 修改这个指针指向的结构体，而不是修改指针本身。因为这个指针 d 是一个副本。
}
