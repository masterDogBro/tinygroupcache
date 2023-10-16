package geecache

import (
	"geecache/lru"
	"sync"
)

// cache 将lru.Cache的方法封装为单机可并行的（可对外部开放的主要是Add和Get两个方法）
type cache struct {
	mu         sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

// add lru.Add的封装，由于cache本身没有初始化函数，故将lru为nil时lru的创建一起封装
// 这被称为延迟初始化(Lazy Initialization)
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil { // cache本身没有
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

// get
func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	} else {
		if v, ok := c.lru.Get(key); ok {
			// value类型转换
			return v.(ByteView), ok
		}
		return
	}
}
