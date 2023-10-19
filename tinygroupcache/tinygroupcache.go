package tinygroupcache

import (
	"fmt"
	"log"
	"sync"
)

// Getter Getter接口，含义一个方法：回调函数Get
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 函数类型，定义其参数和返回值格式
type GetterFunc func(key string) ([]byte, error)

// Get GetterFunc类型的方法Get
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

//如此实现接口Getter是为了接口使用的便利性
//	func GetFromSource(getter Getter, key string) []byte {
//		buf, err := getter.Get(key)
//		if err == nil {
//			return buf
//		}
//		return nil
//	}
//比如以上函数，getter既可以传入GetterFunc类型的匿名函数或者可以转化为GetterFunc类型的普通函数，
//还可以传入实现了Get方法的结构体。

// Group 某一个缓存的命名空间
type Group struct {
	name      string     // 唯一名称
	getter    Getter     // 缓存未命中时调用的回调接口
	mainCache cache      // 并发缓存
	peers     PeerPicker // 负责选取应该访问的其他对等缓存节点的PeerPicker
}

var (
	mu     sync.RWMutex              // 全局/系统读写锁
	groups = make(map[string]*Group) // group字典，保存系统所有Group地址
)

// NewGroup Group的创建/初始化函数，返回一个Group指针
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil getter") // 当错误条件（我们所测试的代码）很严苛且不可恢复，程序不能继续运行时，可以使用 panic() 函数产生一个中止程序的运行时错误
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g
	return g
}

// GetGroup 从groups字典中获得需要的Group指针
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name] // 存在获得nil的情况
	mu.RUnlock()
	return g
}

// RegisterPeers 为Group的peers（负责选取应该访问的其他对等缓存节点的PeerPicker）
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// Get 从缓存系统中获取缓存值，并对缓存命中，未命中情况处理（目前未命中只有单机实现）
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("nil key")
	}

	// 缓存命中
	if v, ok := g.mainCache.get(key); ok {
		log.Printf("key: %s hit cache", key)
		return v, nil
	}

	// 缓存未命中
	return g.load(key)
}

// load 当所需缓存值根据一致性哈希在本节点上时，则从本节点数据源获，并加入本节点的该缓存空间(Group.name)；
// 当在其他对等节点上时，则从对等节点获取但并不加入本节点缓存空间
// TODO 这种缓存加载的形式是否合理，或者说一致性哈希是否该允许缓存迁移？
func (g *Group) load(key string) (value ByteView, err error) { // 直接引入返回值的变量名
	if g.peers != nil {
		if peer, ok := g.peers.PickPeer(key); ok {
			if value, err = g.getFromPeer(peer, key); err == nil {
				return value, nil
			}
			log.Println("Failed to get from peer", err)
		}
	}
	// 如果一致性哈希选择的是本机或者从对等节点获取失败（缓存穿透）
	// 单机情况，只需要从本节点数据源获取
	return g.getLocally(key)
}

// getFromPeer 从某个对等缓存节点中获取当前缓存空间(Group.name)中key对应的缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytesValue, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytesValue)} // 注意深拷贝，否则后续对value.b操作可能会影响到原始缓存
	g.populateCache(key, value)                  //用这个函数而不直接使用g.mainCache.add可能是为了分布式扩展
	return value, nil
}

// populateCache 在g.mainCache添加新键值对
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
