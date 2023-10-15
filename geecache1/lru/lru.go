package lru

import "container/list"

// Cache LRU键值对缓存结构体（单机、非并发）
type Cache struct {
	maxBytes  int64                         // 允许使用的最大内存，maxBytes为0时即缓存大小无上限
	usedBytes int64                         // 当前已使用的内存
	ll        *list.List                    // 实际存储记录双向链表
	cache     map[string]*list.Element      // 字典，kv类型分别为string和链表节点
	OnEvicted func(key string, value Value) // 回调函数，当记录被移除时执行，可以为nil
}

// entry 键值对entry是双线链表节点的数据类型，
// 实际的键值对的value中既包括了value也包括了key（目的是方便淘汰队首节点时，能用key将字典值中的映射也删除）
type entry struct {
	key   string
	value Value
}

// Value 键值对的值为任意实现了Value接口的类型（Value接口只有一个方法Len，用于返回值所占内存大小）
type Value interface {
	Len() int
}

// New Cache实例化函数
func New(maxBytes int64, OnEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: OnEvicted,
	}
}

// Get 通过字典确定双向链表的节点，然后将节点移动到队尾。当缓存被命中执行的逻辑
func (c *Cache) Get(key string) (value Value, ok bool) {
	if element, ok := c.cache[key]; ok {
		c.ll.MoveToFront(element)    // 双向链表将某节点移动到队尾，MoveToFront为移动到队尾，Back和Front是反过来的
		kv := element.Value.(*entry) // element.Value到*entry的类型转化，这里的element并不是上面定义的Value接口
		return kv.value, true
	}
	return
}

// RemoveOldest 缓存淘汰的实际执行逻辑，包括双向链表ll的首节点删除、字典cache中对应键值对删除、更新usedBytes、执行回调函数
func (c *Cache) RemoveOldest() {
	element := c.ll.Back() // 获取双向链表头，Back为获取头
	if element != nil {
		c.ll.Remove(element)
		kv := element.Value.(*entry)
		delete(c.cache, kv.key)
		c.usedBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil { // 回调函数不为空则执行
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Add 当键存在，更新键值移到队尾（已有的缓存被更新）；当键不存在，在双向队列添加新纪录（缓存内不存在，从数据库访问，并写入缓存）
// 同时更新usedBytes，并按usedBytes情况查看是否需要执行缓存淘汰
func (c *Cache) Add(key string, value Value) {
	if element, ok := c.cache[key]; ok {
		c.ll.MoveToFront(element)
		kv := element.Value.(*entry)
		c.usedBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value //因为kv实际上是键值对的指针，所以可以直接更改kv.value的值
	} else {
		element := c.ll.PushFront(&entry{
			key,
			value,
		})
		c.cache[key] = element
		c.usedBytes += int64(len(key)) + int64(value.Len())
	}
	for c.maxBytes != 0 && c.maxBytes < c.usedBytes { //当新增或者修改缓存中的键值对后，超出了Cache容量时，执行缓存淘汰直到有空余容量
		// c.maxBytes为0时代表缓存无上限
		c.RemoveOldest()
	}
}

// Len 返回双向链表中被添加了多少条记录（而不是键值对数据总长度）
func (c *Cache) Len() int {
	return c.ll.Len()
}

// TODO
// [LRU] what if a single element's size exceeded the max bytes of LRU? #1
// 具体到我写的这段LRU缓存结构体上，过大的元素会实际地插入双线链表，最后又因maxBytes的检查而弹出该元素，这导致链表插入的时间和为链表节点分配的内存空间被浪费。
// 如果为entry.Value的值添加限制，并在修改缓存中已有键值对或者新增键值对前进行检查，因该情形产生的资源浪费应该能够被避免。
