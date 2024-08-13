package consumer

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	consistenthash "git.woa.com/kefuai/mini-router/consumer/impl/algorithm/hash"
	"git.woa.com/kefuai/mini-router/consumer/impl/algorithm/random"
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
	discoverServiceAddr = "localhost:5200"
	pullInterval        = 5
)

type Consumer struct {
	config       atomic.Pointer[routingpb.Group]
	ctx          context.Context
	cancel       context.CancelFunc
	routingTable *routingpb.Group
	mu           sync.RWMutex
	version      int64
	virtualNode  int

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
	mlog.Debug("load config successfully", zap.Any("config", group))

	c.ctx, c.cancel = context.WithCancel(context.Background())
	if err := c.grpcConnect(); err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (c *Consumer) watchLoop() {
	ticker := time.NewTicker(pullInterval * time.Second)
	mlog.Debug("start to run watch loop")
	for {
		select {
		case <-ticker.C:
			c.updateRoutingTable()
		case <-c.ctx.Done():
			mlog.Info("watch loop stopped")
			return
		}
	}
}

func (c *Consumer) getConfig() *routingpb.Group {
	return c.config.Load()
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
func (c *Consumer) coverRoutingTable(routingTable *routingpb.Group, version int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routingTable = routingTable
	c.version = version
	mlog.Info("cover routing table successfully", zap.Any("routing table", routingTable), zap.Any("version", version))
}

// TODO: 增量更新
func (c *Consumer) processRoutingTable(log *routingpb.ChangeRecords, version int64) {

}

// 全量更新哈希环
func (c *Consumer) updatehashRing() {
	hashRing := make(map[string]*consistenthash.HashRing)

	c.mu.RLock()
	for _, host := range c.routingTable.GetHosts() {
		hostRing := consistenthash.NewHashRing(c.virtualNode)
		if host.GetRoutingRule().GetLb() == routingpb.LoadBalancer_consistent_hash {
			for _, endpoint := range host.GetEndpoints() {
				hostRing.AddNode(combineAddr(endpoint.GetIp(), endpoint.GetPort()))
			}
			hashRing[host.GetName()] = hostRing
		}
	}
	c.mu.RUnlock()

	c.hashRing.Store(&hashRing)
}

func (c *Consumer) consistenthashRouting(hostName string, key string, target string) string {
	ring := *c.hashRing.Load()
	if _, ok := ring[hostName]; !ok {
		return ""
	}
	return ring[hostName].GetNode(strings.TrimPrefix(key, target))
}

func (c *Consumer) updateRoutingTable() {
	config := c.getConfig()
	resp, err := c.discoverClient.ConsumerUpdate(c.ctx, &consumerpb.ConsumerUpdateRequest{
		GroupName: config.GetName(),
		HostName:  lo.Keys(config.GetHosts()),
		Version:   c.version,
	})
	if err != nil {
		mlog.Warnf("failed to update routing table: %v", err)
		return
	}
	if resp.Outdated {
		mlog.Infof("routing table outdated, start to cover old version")
		c.coverRoutingTable(resp.GetGroup(), resp.GetVersion())
	} else {
		c.processRoutingTable(resp.GetChanges(), resp.GetVersion())
	}
}

func (c *Consumer) getTargetByKey(hostName string, key string) (string, error) {
	routing := c.config.Load()
	if _, ok := routing.GetHosts()[hostName]; !ok {
		return "", util.ErrorfWithPos("no such host [%v] in routing table", hostName)
	}
	host := routing.GetHosts()[hostName]

	if host.GetRoutingRule().GetLb() == routingpb.LoadBalancer_consistent_hash {
		return c.consistenthashRouting(hostName, key, host.GetRoutingRule().GetTarget()), nil
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
	return "", util.ErrorfWithPos("no match rules, expect: %v, actual: %v", strategy, key)
}

func (c *Consumer) getTargetByConfig(hostName string) (string, error) {
	routing := c.config.Load()
	if _, ok := routing.GetHosts()[hostName]; !ok {
		return "", util.ErrorfWithPos("no such host [%v] in routing table", hostName)
	}
	host := routing.GetHosts()[hostName]

	var targetAddr string
	switch host.GetRoutingRule().GetLb() {
	case routingpb.LoadBalancer_random:
		targetAddr = c.randomRouting(lo.MapToSlice(host.GetEndpoints(), func(eid int64, endpoint *routingpb.Endpoint) string {
			return combineAddr(endpoint.GetIp(), endpoint.GetIp())
		}))
	case routingpb.LoadBalancer_weight:
		targetAddr = c.weightRouting(lo.MapToSlice(host.GetEndpoints(), func(eid int64, endpoint *routingpb.Endpoint) *routingpb.Endpoint {
			return endpoint
		}))
	case routingpb.LoadBalancer_target:
		targetAddr = host.GetRoutingRule().GetTarget()
	}

	return targetAddr, nil
}

func (c *Consumer) randomRouting(addrs []string) string {
	index := random.Intn(len(addrs))
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

func parseAddr(addr string) ([]string, error) {
	resp := strings.Split(addr, ":")
	if len(resp) != 2 {
		return nil, util.ErrorfWithPos("wrong addr format: %v", addr)
	}
	return resp, nil
}
