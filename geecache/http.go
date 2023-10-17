package geecache

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const defaultBasePath = "/_geecache/" //节点通信地址前缀

// HTTPPool 为 HTTP 对等体池实现了 PeerPicker。暂时没懂啥意思，看后续怎么用
type HTTPPool struct {
	self     string // 主机地址/IP:端口号
	bashPath string // 节点间通讯地址的前缀默认值采用defaultBasePath
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		bashPath: defaultBasePath,
	}
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
