package geecache

// ByteView 缓存值结构体，只读
type ByteView struct {
	b []byte // 缓存的真实值，为支持任意类型的数据类型的存储采用byte类型
}

// Len 由于Value接口的需要，ByteView必须实现Len方法
func (bv ByteView) Len() int {
	return len(bv.b)
}

// ByteSlice ByteView只读，为防止修改只返回拷贝值
func (bv ByteView) ByteSlice() []byte {
	return cloneBytes(bv.b)
}

// cloneBytes 深复制，返回b的拷贝
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// String 返回bv.b的字符串形式
func (bv ByteView) String() string {
	return string(bv.b)
}
