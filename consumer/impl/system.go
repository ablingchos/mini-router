package consumer

import (
	"context"
	"encoding/json"
	"math/rand"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	consistenthash "git.woa.com/kefuai/mini-router/consumer/impl/algorithm/hash"
	"git.woa.com/kefuai/mini-router/consumer/impl/algorithm/weight"
	"git.woa.com/kefuai/mini-router/pkg/common"
	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	once     sync.Once
	consumer *Consumer
)

const (
	metricsAddr         = "localhost:6060"
	discoverServiceAddr = "localhost:5200"
	pullInterval        = 5
)

type Consumer struct {
	config       atomic.Pointer[routingpb.Group]
	groupName    string
	hostName     []string
	ctx          context.Context
	cancel       context.CancelFunc
	routingTable *routingpb.Group
	mu           sync.RWMutex
	virtualNode  int
	metrics      *Metrics

	hashRing atomic.Pointer[map[string]*consistenthash.HashRing]

	discoverClient consumerpb.ConsumerServiceClient
}

func (c *Consumer) initializeConsumer(configPath string) error {
	// 读配置
	bytes, err := common.LoadYAML(configPath)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	if len(bytes) != 1 {
		return util.ErrorfWithPos("wrong length of config: %v", len(bytes))
	}

	mlog.Debug("read config", zap.Any("config", string(bytes[0])))
	group := &routingpb.Group{}
	if err := json.Unmarshal(bytes[0], group); err != nil {
		return util.ErrorWithPos(err)
	}
	c.config.Store(group)
	c.groupName = group.GetName()
	c.hostName = lo.Keys(group.GetHosts())
	mlog.Debug("load config successfully", zap.Any("config", group))

	c.ctx, c.cancel = context.WithCancel(context.Background())
	if err := c.grpcConnect(); err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (c *Consumer) grpcConnect() error {
	conn, err := grpc.NewClient(discoverServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	c.discoverClient = consumerpb.NewConsumerServiceClient(conn)
	return nil
}

// 全量覆盖
func (c *Consumer) initRoutingTable(routingTable *routingpb.Group) {
	config := c.config.Load()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routingTable = routingTable

	for hostName, host := range config.GetHosts() {
		if _, ok := c.routingTable.GetHosts()[hostName]; !ok {
			mlog.Panicf("no such host: %v", hostName)
		}
		c.routingTable.Hosts[hostName].RoutingRule = &routingpb.RoutingRule{
			Lb:     host.GetRoutingRule().GetLb(),
			Target: host.GetRoutingRule().GetTarget(),
		}
		c.routingTable.Hosts[hostName].UserRule = &routingpb.UserRule{
			MatchRule:   host.GetUserRule().GetMatchRule(),
			Destination: host.GetUserRule().GetDestination(),
		}
	}

	go c.inithashRing()
	go c.streamWatch()
	mlog.Debug("init routing table successfully", zap.Any("routing table", routingTable))
}

func (c *Consumer) reportMetrics() {
	c.metrics = NewMetrics("consumer")
	c.metrics.Start(metricsAddr)
}

// 全量更新哈希环
func (c *Consumer) inithashRing() {
	hashRing := make(map[string]*consistenthash.HashRing)
	routing := c.routingTable

	c.mu.RLock()
	for name, host := range routing.GetHosts() {
		hostRing := consistenthash.NewHashRing(c.virtualNode)
		if host.GetRoutingRule().GetLb() == routingpb.LoadBalancer_consistent_hash {
			for _, endpoint := range c.routingTable.GetHosts()[name].GetEndpoints() {
				hostRing.AddNode(strconv.Itoa(int(endpoint.GetEid())))
			}
			hashRing[name] = hostRing
		}
	}
	c.mu.RUnlock()

	c.hashRing.Store(&hashRing)
}

func (c *Consumer) consistenthashRouting(hostName string, key string, target string) (string, error) {
	ring := *(c.hashRing.Load())
	if _, ok := ring[hostName]; !ok {
		return "", util.ErrorfWithPos("no such host")
	}
	if !strings.HasPrefix(key, target) {
		return "", util.ErrorfWithPos("wrong prefix, need: %v, actual: %v", target, key)
	}
	return ring[hostName].GetNode(key), nil
}

func (c *Consumer) streamWatch() {
	req := &consumerpb.RoutingChangeRequest{}
	stream, err := c.discoverClient.RoutingChange(c.ctx, req)
	if err != nil {
		mlog.Errorf("failed to create stream: %v", err)
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			mlog.Errorf("failed to receive from stream: %v", err)
			return
		}
		c.processStream(resp)
	}
}

func (c *Consumer) processStream(resp *consumerpb.RoutingChangeReply) error {
	config := c.config.Load()

	added := make([]string, 0)
	delete := make([]string, 0)
	for _, key := range resp.GetAddedEndpoints() {
		groupName, hostName, eid, err := parseKey(key)
		if err != nil {
			mlog.Errorf("failed to parse key: %v", err)
			continue
		}
		if groupName != config.GetName() || config.GetHosts()[hostName] == nil {
			continue
		}
		added = append(added, groupName+"/"+hostName+"/"+eid)
	}
	c.processAddEndpoint(added)

	for _, key := range resp.GetDeletedEndpoints() {
		groupName, hostName, eid, err := parseKey(key)
		if err != nil {
			mlog.Errorf("failed to parse key: %v", err)
			continue
		}
		if groupName != config.GetName() || config.GetHosts()[hostName] == nil {
			continue
		}
		delete = append(delete, hostName+"/"+eid)
	}
	c.processDeleteEndpoint(delete)

	return nil
}

func (c *Consumer) processAddEndpoint(addKey []string) {
	resp, err := c.discoverClient.PullEndpoint(c.ctx, &consumerpb.PullEndpointRequest{
		Endpoints: addKey,
	})
	if err != nil {
		mlog.Errorf("failed to add endpoints: %v", err)
		return
	}

	for key, endpoint := range resp.GetEndpoints() {
		_, hostName, _, err := parseKey(key)
		if err != nil {
			mlog.Errorf("failed to parse key: %v", err)
			continue
		}
		c.insertEndpoint(hostName, endpoint)
	}
}

func (c *Consumer) insertEndpoint(hostName string, endpoint *routingpb.Endpoint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	routing := c.routingTable
	if _, ok := routing.GetHosts()[hostName]; !ok {
		mlog.Debugf("host not exists: %v", hostName)
		return
	}
	host := routing.GetHosts()[hostName]

	if _, ok := host.GetEndpoints()[endpoint.GetEid()]; ok {
		mlog.Debugf("endpoint exists: %v", endpoint)
		return
	}

	host.Endpoints[endpoint.GetEid()] = endpoint

	hash := *(c.hashRing.Load())
	hash[hostName].AddNode(strconv.Itoa(int(endpoint.GetEid())))
	c.hashRing.Store(&hash)
	mlog.Infof("delete offline endpoint [%v %v]", hostName, endpoint.GetEid())
}

func (c *Consumer) processDeleteEndpoint(deleteKey []string) {
	hash := *(c.hashRing.Load())
	c.mu.Lock()
	defer c.mu.Unlock()
	routing := c.routingTable
	for _, key := range deleteKey {
		parts := strings.Split(key, "/")
		eid, err := strconv.Atoi(parts[1])
		if err != nil {
			mlog.Errorf("wrong format: %v", key)
			continue
		}
		delete(routing.GetHosts()[parts[0]].GetEndpoints(), int64(eid))
		hash[parts[0]].RemoveNode(parts[1])
		mlog.Infof("delete offline endpoint [%v %v]", parts[0], eid)
	}

	c.hashRing.Store(&hash)
}

func parseKey(key string) (string, string, string, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", "", util.ErrorfWithPos("wrong format of key: %v", key)
	}
	return parts[0], parts[1], parts[2], nil
}

func (c *Consumer) getTargetByKey(hostName string, key string) (string, error) {
	defer func() {
		c.metrics.incrQuestNumber()
	}()

	c.mu.RLock()
	defer c.mu.RUnlock()
	routing := c.routingTable
	if _, ok := routing.GetHosts()[hostName]; !ok {
		c.metrics.incrFailNumber()
		return "", util.ErrorfWithPos("no such host [%v] in routing table", hostName)
	}
	host := routing.GetHosts()[hostName]

	if host.GetRoutingRule().GetLb() == routingpb.LoadBalancer_consistent_hash {
		eidStr, err := c.consistenthashRouting(hostName, key, host.GetRoutingRule().GetTarget())
		if err != nil {
			return "", util.ErrorWithPos(err)
		}
		if eidStr == "" {
			return "", util.ErrorfWithPos("target host is empty")
		}
		eid, err := strconv.Atoi(eidStr)
		if err != nil {
			return "", util.ErrorWithPos(err)
		}
		endpoint := host.GetEndpoints()[int64(eid)]
		return combineAddr(endpoint.GetIp(), endpoint.GetPort()), nil
	}

	strategy := host.GetUserRule()
	switch strategy.GetMatchRule().GetMatch() {
	case routingpb.Match_prefix:
		if strings.HasPrefix(key, strategy.GetMatchRule().GetContent()) {
			return combineAddr(strategy.GetDestination().GetIp(), strategy.GetDestination().GetPort()), nil
		}
	case routingpb.Match_exact:
		if key == strategy.GetMatchRule().GetContent() {
			return combineAddr(strategy.GetDestination().GetIp(), strategy.GetDestination().GetPort()), nil
		}
	}
	c.metrics.incrFailNumber()
	return "", util.ErrorfWithPos("no match rules, expect: %v, actual: %v", strategy, key)
}

func (c *Consumer) getTargetByConfig(hostName string) (string, error) {
	defer func() {
		c.metrics.incrQuestNumber()
	}()
	c.mu.RLock()
	defer c.mu.RUnlock()

	routing := c.routingTable
	if _, ok := routing.GetHosts()[hostName]; !ok {
		c.metrics.incrFailNumber()
		return "", util.ErrorfWithPos("no such host [%v] in routing table", hostName)
	}

	hostRouting := routing.GetHosts()[hostName]
	var targetAddr string
	switch hostRouting.GetRoutingRule().GetLb() {
	case routingpb.LoadBalancer_random:
		targetAddr = c.randomRouting(lo.MapToSlice(hostRouting.GetEndpoints(), func(eid int64, endpoint *routingpb.Endpoint) string {
			return combineAddr(endpoint.GetIp(), endpoint.GetPort())
		}))
	case routingpb.LoadBalancer_weight:
		targetAddr = c.weightRouting(lo.MapToSlice(hostRouting.GetEndpoints(), func(eid int64, endpoint *routingpb.Endpoint) *routingpb.Endpoint {
			return endpoint
		}))
	case routingpb.LoadBalancer_target:
		targetAddr = hostRouting.GetRoutingRule().GetTarget()
	}

	return targetAddr, nil
}

func (c *Consumer) randomRouting(addrs []string) string {
	index := rand.Intn(len(addrs))
	return addrs[index]
}

func (c *Consumer) weightRouting(endpoints []*routingpb.Endpoint) string {
	weightSlice := lo.Map(endpoints, func(item *routingpb.Endpoint, index int) int64 {
		return item.GetWeight()
	})
	index := weight.GetEndpoint(weightSlice)
	endpoint := endpoints[index]
	return combineAddr(endpoint.GetIp(), endpoint.GetPort())
}

func combineAddr(ip string, port string) string {
	return ip + ":" + port
}
