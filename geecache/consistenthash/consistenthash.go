package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// Hash 定义Hash函数类型
type Hash func(data []byte) uint32

// Map 一致性哈希算法的主数据结构
type Map struct {
	mu           sync.Mutex // 确保缓存节点添加、删除可并行
	hash         Hash       // 一致性哈希所用哈希函数，默认使用crc32.ChecksumIEEE
	copyMultiple int        // 虚拟节点倍数即一个真实缓存节点对应多少虚拟几点
	keys         []int      // 哈希环，有序的（从大到小），保存虚拟节点的哈希值
	// Map.Get中二分查找依赖有序实现。
	hashMap map[int]string // 字典，存储真实节点与虚拟节点的映射关系
}

func NewMap(copyMultiple int, hf Hash) *Map {
	m := &Map{
		hash:         hf,
		copyMultiple: copyMultiple,
		hashMap:      make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add 添加缓存节点
func (m *Map) Add(keys ...string) {
	m.mu.Lock()
	// TODO 以锁的形式完全限制m.keys，m.hashMap可能会严重影响性能，后续可以研究其他一致性哈希算法实现
	defer m.mu.Unlock()
	for _, key := range keys {
		// key是真实缓存节点名称
		for i := 0; i < m.copyMultiple; i++ {
			hashValue := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hashValue)
			m.hashMap[hashValue] = key
		}
	}
	sort.Ints(m.keys)
}

// Del 删除名称对应缓存节点/虚拟节点
func (m *Map) Del(key string) {
	m.mu.Lock()
	// TODO 以锁的形式完全限制m.keys，m.hashMap可能会严重影响性能，后续可以研究其他一致性哈希算法实现
	defer m.mu.Unlock()
	for i := 0; i < m.copyMultiple; i++ {
		hashValue := int(m.hash([]byte(strconv.Itoa(i) + key)))
		idx := sort.SearchInts(m.keys, hashValue)
		if idx == len(m.keys) {
			// 找不到对应缓存节点，直接返回
			return
		}
		// 被gpt骗了
		//if idx < len(m.keys)-1 {
		//	m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
		//} else {
		//	m.keys = m.keys[:idx]
		//}
		m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
	}
}

// Get 根据key值选出需要访问的缓存节点
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hashValue := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		// 因为sort.Search使用的比较函数是keys[i] >= hashValue，我们能找到第一个"m.keys[i] >= hashValue"的i
		// 当这个i不存在时，idx获得的值为len(m.keys)，这就需要取余了，因为从某个虚拟节点逆时针一侧(不跨越哈希环起终点时，哈希值小于它本身)开始
		// 直到下一个虚拟节点结束的哈希环才是它的对应缓存Key的哈希值范围。
		return m.keys[i] >= hashValue
	})
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
