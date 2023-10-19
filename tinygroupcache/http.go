package tinygroupcache

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"tinygroupcache/consistenthash"
)

const defaultBasePath = "/_tgcache/" //节点通信地址前缀
const defaultCopyMultiple = 3

// HTTPPool 为 HTTP 对等体池实现了 PeerPicker。
// 基于一致性hash来实现缓存节点选择
type HTTPPool struct {
	self        string                 // 主机地址/IP:端口号
	bashPath    string                 // 节点间通讯地址的前缀默认值采用defaultBasePath
	mu          sync.Mutex             // 互斥锁的守护对象是peers和httpGetters
	peers       *consistenthash.Map    // 一致性哈希算法实体
	httpGetters map[string]*httpGetter // 保存httpGetter的字典，其key为缓存节点地址如："http://10.0.0.2:8008"
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		bashPath: defaultBasePath,
	}
}

// Set 实例化一致性哈希，并将缓存节点地址添加到HTTPPool.peers和HTTPPool.httpGetters
// TODO 这种初始化形式后续没有留有Peers添加或者删除的扩展余地，但在consistenthash中是有相应的方法的
func (hp *HTTPPool) Set(peers ...string) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.peers = consistenthash.NewMap(defaultCopyMultiple, nil)
	hp.peers.Add(peers...)
	hp.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		hp.httpGetters[peer] = &httpGetter{baseURL: peer + hp.bashPath}
	}
}

// PickPeer 根据所寻找缓存的key，来确定缓存节点并返回其PeerGetter。如果没有找到或者找到的是自己，则返回nil
func (hp *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if peer := hp.peers.Get(key); peer != "" && peer != hp.self { // 确保了不会返回自己而陷入循环
		hp.Log("Pick peer %s", peer)
		return hp.httpGetters[peer], true
	}
	return nil, false
}

// Log HTTPPool内部日志函数
func (hp *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", hp.self, fmt.Sprintf(format, v...))
}

func (hp *HTTPPool) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, hp.bashPath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	hp.Log("%s %s", r.Method, r.URL.Path)
	// url path 格式应为 /<basepath>/<groupname>/<key>
	parts := strings.SplitN(r.URL.Path[len(hp.bashPath):], "/", 2)
	if len(parts) != 2 { // url path 格式错误
		http.Error(writer, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)
	if group == nil { // 缓存对象示例不存在
		http.Error(writer, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key)
	if err != nil { // 缓存查询报错
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	// 未发生任何错误，返回缓存值
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, errW := writer.Write(view.ByteSlice())
	if errW != nil {
		hp.Log("%s", errW)
	}
}

type httpGetter struct {
	baseURL string
}

func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	getUrl := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(group),
		url.QueryEscape(key),
	)
	res, errG := http.Get(getUrl)
	if errG != nil {
		return nil, errG
	}

	// GoLand提示我“未封装错误”，小改一下
	defer func(Body io.ReadCloser) {
		errC := Body.Close()
		if errC != nil {
			return
		}
	}(res.Body)

	// 处理服务器返回错误
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}

	// 处理Response读取错误
	bytes, errR := io.ReadAll(res.Body)
	if errR != nil {
		return nil, fmt.Errorf("reading response body: %v", errR)
	}

	return bytes, nil
}

// 接口断言，用于检查 httpGetter 类型是否实现了 PeerGetter 接口。
var _ PeerGetter = (*httpGetter)(nil)
