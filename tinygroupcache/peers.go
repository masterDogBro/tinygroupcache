package tinygroupcache

import pb "tinygroupcache/cachepb"

// PeerPicker PickPeer方法基于key选择对应缓存节点peer，并返回其PeerGetter
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter Get方法从对应Group查找key的缓存值并返回
type PeerGetter interface {
	// Get 第一种实现 Get(group string, key string) ([]byte, error)
	Get(in *pb.Request, out *pb.Response) error
}
