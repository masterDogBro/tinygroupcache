package tinygroupcache

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"sync"
	pb "tinygroupcache/cachepb"
	"tinygroupcache/consistenthash"
)

// GrpcPool 向前继承pb.UnimplementedGroupCacheServer，为 RPC 对等体池实现了 PeerPicker。
type GrpcPool struct {
	pb.UnimplementedGroupCacheServer

	self        string
	mu          sync.Mutex
	peers       *consistenthash.Map
	grpcGetters map[string]*grpcGetter
}

func NewGrpcPool(self string) *GrpcPool {
	return &GrpcPool{
		self:        self,
		peers:       consistenthash.NewMap(defaultCopyMultiple, nil),
		grpcGetters: map[string]*grpcGetter{},
	}
}

// Set 实例化一致性哈希，并将缓存节点地址添加到RPCPool.peers和RPCPool.grpcGetters
// TODO 这种初始化形式后续没有留有Peers添加或者删除的扩展余地，但在consistenthash中是有相应的方法的
func (p *GrpcPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers.Add(peers...)
	for _, peer := range peers {
		p.grpcGetters[peer] = &grpcGetter{
			addr: peer,
		}
	}
}

func (p *GrpcPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		return p.grpcGetters[peer], true
	}
	return nil, false
}

// 接口断言
var _ PeerPicker = (*GrpcPool)(nil)

func (p *GrpcPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// Get 实现GrpcPool的Get方法，类似ServeHTTP的逻辑，但其使用被封装在cachepb_grpc.pb.go里了
func (p *GrpcPool) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	p.Log("%s %s", in.Group, in.Key)
	response := &pb.Response{}

	group := GetGroup(in.Group)
	if group == nil {
		p.Log("no such group %v", in.Group)
		return response, fmt.Errorf("no such group %v", in.Group)
	}
	value, err := group.Get(in.Key)
	if err != nil {
		p.Log("get key %v error %v", in.Key, err)
		return response, err
	}

	response.Value = value.ByteSlice()
	return response, nil
}

// Run gRPC服务器启动
func (p *GrpcPool) Run() {
	lis, err := net.Listen("tcp", p.self)
	if err != nil {
		panic(err)
	}

	server := grpc.NewServer()
	pb.RegisterGroupCacheServer(server, p)

	reflection.Register(server)
	err = server.Serve(lis)
	if err != nil {
		panic(err)
	}
}

// grpcGetter 实际上作为gRPC客户端
type grpcGetter struct {
	addr string
}

func (g *grpcGetter) Get(in *pb.Request, out *pb.Response) error {
	c, err := grpc.Dial(g.addr, grpc.WithInsecure())
	if err != nil {
		return err
	}
	client := pb.NewGroupCacheClient(c)
	response, err := client.Get(context.Background(), in)
	out.Value = response.Value
	return err
}

// 接口断言
var _ PeerGetter = (*grpcGetter)(nil)
