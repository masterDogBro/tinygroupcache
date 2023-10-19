package tinygroupcache

// PeerPicker PickPeer方法基于key选择对应缓存节点peer，并返回其PeerGetter
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter Get方法从对应Group查找key的缓存值并返回
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
}
