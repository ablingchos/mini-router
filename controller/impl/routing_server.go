package controller

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	redis "github.com/go-redis/redis/v8"
	"github.com/samber/lo"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	pullInterval      = 3
	routingServerPort = ":5200"
	cacheSize         = 100
)

type RoutingServer struct {
	etcdClient  *clientv3.Client
	redisClient *redis.Client
	// routingTable atomic.Pointer[routingpb.RoutingTable]
	routingCache []string
	cacheMap     map[string]*routingpb.Host
	ctx          context.Context
	cancelFunc   context.CancelFunc
	streams      []consumerpb.ConsumerService_RoutingChangeServer
	mu           sync.RWMutex
	metrics      *Metrics

	consumerpb.UnimplementedConsumerServiceServer
}

func NewRoutingServer() (*RoutingServer, error) {
	server := &RoutingServer{
		routingCache: make([]string, 0, cacheSize),
		cacheMap:     make(map[string]*routingpb.Host, cacheSize),
		metrics:      NewMetrics("routing_server"),
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdUri},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}

	redis := redis.NewClient(&redis.Options{
		Addr:     redisUri,
		Password: "",
		DB:       0,
	})

	server.redisClient = redis
	server.etcdClient = client
	ctx, cancel := context.WithCancel(context.Background())
	server.ctx = ctx
	server.cancelFunc = cancel
	return server, nil
}

func (s *RoutingServer) Run() {
	go s.metrics.Start("localhost" + routingServerPort)
	go s.serveGrpc()
	go s.watchLoop()
}

func (s *RoutingServer) Stop() {
	s.cancelFunc()
}

func (s *RoutingServer) serveGrpc() {
	lis, err := net.Listen("tcp", routingServerPort)
	if err != nil {
		mlog.Fatalf("failed to start grpc server: %v", err)
	}

	server := grpc.NewServer()
	consumerpb.RegisterConsumerServiceServer(server, s)
	mlog.Infof("server listening on port: %v", lis.Addr())
	if err := server.Serve(lis); err != nil {
		mlog.Fatalf("failed to serve: %v", err)
	}
}

func (s *RoutingServer) getRedis(groupName string, hostName string) (*routingpb.Host, error) {
	hostKey := groupName + "/" + hostName
	resp, err := s.redisClient.HGetAll(s.ctx, hostKey).Result()
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	host := &routingpb.Host{
		Name:      hostName,
		Endpoints: make(map[int64]*routingpb.Endpoint, len(resp)),
	}
	for eidStr, value := range resp {
		eid, err := strconv.Atoi(eidStr)
		if err != nil {
			mlog.Errorf("failed to convert string to int: %v, str: %v", err, eidStr)
			continue
		}

		endpoint := &routingpb.Endpoint{}
		if err := json.Unmarshal([]byte(value), endpoint); err != nil {
			mlog.Warnf("failed to unmarshal: %v", value)
		}
		host.Endpoints[int64(eid)] = endpoint
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.routingCache) >= cacheSize {
		deleteKey := s.routingCache[0]
		s.routingCache = s.routingCache[1:]
		delete(s.cacheMap, deleteKey)
	}
	s.routingCache = append(s.routingCache, hostKey)
	s.cacheMap[hostKey] = host
	return host, nil
}

func (s *RoutingServer) RoutingChange(req *consumerpb.RoutingChangeRequest, stream consumerpb.ConsumerService_RoutingChangeServer) error {
	// 将新连接的客户端添加到 streams 列表中
	s.mu.Lock()
	s.streams = append(s.streams, stream)
	s.mu.Unlock()

	// 等待客户端断开连接
	<-stream.Context().Done()

	// 从 streams 列表中移除断开连接的客户端
	s.mu.Lock()
	lo.DropWhile(s.streams, func(item consumerpb.ConsumerService_RoutingChangeServer) bool {
		return item == stream
	})
	s.mu.Unlock()

	return nil
}

func (s *RoutingServer) watchLoop() {
	watchChan := s.etcdClient.Watch(s.ctx, routingHeartbeatKey, clientv3.WithPrefix())
	for watchResp := range watchChan {
		addedEndpoints := make(map[string]*routingpb.Endpoint, 0)
		deletedEndpoints := make(map[string]struct{}, 0)
		for _, event := range watchResp.Events {
			key := strings.TrimPrefix(string(event.Kv.Key), "/")
			groupName, hostName, eidStr, err := parseHeartbeatKey(key)
			if err != nil {
				mlog.Warn("wrong heartbeat key format", zap.Any("key", key))
				continue
			}
			mlog.Debugf("Received etcd event: %v", event)
			switch event.Type {
			case mvccpb.PUT:
				endpoint := &routingpb.Endpoint{}
				if err := json.Unmarshal(event.Kv.Value, endpoint); err != nil {
					mlog.Errorf("failed to unmarshal endpoint: %v", event.Kv.Value)
					break
				}
				if event.IsCreate() {
					addedEndpoints[groupName+"/"+hostName+"/"+eidStr] = endpoint
					s.addLocalCache(groupName+"/"+hostName, endpoint.GetEid(), endpoint)
				}
			case mvccpb.DELETE:
				eid, _ := strconv.Atoi(eidStr)
				deletedEndpoints[groupName+"/"+hostName+"/"+eidStr] = struct{}{}
				s.deleteLocalCache(groupName+"/"+hostName, int64(eid))
			default:
				mlog.Errorf("invalid type of etcd event: %v", event)
			}
		}
		// 向sdk广播消息
		s.mu.RLock()
		for _, stream := range s.streams {
			if err := stream.Send(&consumerpb.RoutingChangeReply{
				DeletedEndpoints: lo.Keys(deletedEndpoints),
				AddedEndpoints:   lo.Keys(addedEndpoints),
			}); err != nil {
				mlog.Warnf("failed to send to stream: %v", err)
			}
		}
		s.mu.RUnlock()
	}
}

func (s *RoutingServer) PullEndpoint(ctx context.Context, req *consumerpb.PullEndpointRequest) (*consumerpb.PullEndpointReply, error) {
	hosts := []string{}
	eids := make(map[string]string, len(req.GetEndpoints()))
	for _, key := range req.GetEndpoints() {
		lastIndex := strings.LastIndex(key, "/")
		if lastIndex == -1 {
			return nil, util.ErrorfWithPos("wrong req format, key: %v", key)
		}
		eids[key[:lastIndex]] = key[lastIndex+1:]
		hosts = append(hosts, key[:lastIndex])
	}
	hosts = lo.Uniq(hosts)

	host := make(map[string]*routingpb.Host)
	for _, key := range hosts {
		parts := strings.Split(key, "/")
		if len(parts) != 2 {
			return nil, util.ErrorfWithPos("wrong req format, key: %v", key)
		}
		temp, err := s.getHost(parts[0], parts[1])
		if err != nil {
			return nil, util.ErrorWithPos(err)
		}
		host[key] = temp
	}

	resp := &consumerpb.PullEndpointReply{
		Endpoints: make(map[string]*routingpb.Endpoint, len(req.GetEndpoints())),
	}
	for key, eidStr := range eids {
		eid, _ := strconv.Atoi(eidStr)
		resp.Endpoints[key+eidStr] = host[key].GetEndpoints()[int64(eid)]
	}
	return resp, nil
}

func (s *RoutingServer) deleteLocalCache(hostKey string, eid int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cacheMap[hostKey]; ok {
		delete(s.cacheMap[hostKey].GetEndpoints(), eid)
	}
}

func (s *RoutingServer) addLocalCache(hostKey string, eid int64, endpoint *routingpb.Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cacheMap[hostKey]; ok {
		s.cacheMap[hostKey].Endpoints[eid] = endpoint
		return
	}
}

func (s *RoutingServer) ConsumerInit(ctx context.Context, req *consumerpb.ConsumerInitRequest) (*consumerpb.ConsumerInitReply, error) {
	group, err := s.getTargetRouting(ctx, req.GetGroupName(), req.GetHostName())
	if err != nil {
		mlog.Errorf("failed to init consumer: %v", err)
		return nil, util.ErrorWithPos(err)
	}
	return &consumerpb.ConsumerInitReply{
		Group: group,
	}, nil
}

func (s *RoutingServer) getHost(groupName string, hostName string) (*routingpb.Host, error) {
	s.mu.RLock()
	if _, ok := s.cacheMap[groupName+"/"+hostName]; ok {
		s.mu.RUnlock()
		return s.cacheMap[groupName+"/"+hostName], nil
	}

	s.mu.RUnlock()
	host, err := s.getRedis(groupName, hostName)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return host, err
}

func (s *RoutingServer) getTargetRouting(ctx context.Context, groupName string, hosts []string) (*routingpb.Group, error) {
	// recover
	group := &routingpb.Group{
		Name:  groupName,
		Hosts: make(map[string]*routingpb.Host, len(hosts)),
	}
	for _, hostName := range hosts {
		host, err := s.getHost(groupName, hostName)
		if err != nil {
			mlog.Warnf("failed to get host [%v %v]", groupName, hostName)
			continue
		}
		group.Hosts[hostName] = host
	}
	return group, nil
}
