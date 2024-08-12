package consumer

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/kefuai/mini-router/provider/impl/algorithm/random"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v2"
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

	discoverClient consumerpb.ConsumerServiceClient
}

func (c *Consumer) initializeConsumer(configPath string) error {
	// 读配置
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	config := &routingpb.Group{}
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	c.config.Store(config)

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

func (c *Consumer) getTargetByTag(hostName string, tag string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.routingTable.GetHosts()[hostName]; !ok {
		return "", util.ErrorfWithPos("no such host [%v] in routing table", hostName)
	}

	strategy := c.routingTable.GetHosts()[hostName].GetUserRule()
	switch strategy.GetMatchRule().GetMatch() {
	case routingpb.Match_prefix:
		if strings.HasPrefix(tag, strategy.GetMatchRule().GetContent()) {
			return combineAddr(strategy.GetDestination().GetIp(), strategy.GetDestination().GetPort()), nil
		}
	case routingpb.Match_exact:
		if tag == strategy.GetMatchRule().GetContent() {
			return combineAddr(strategy.GetDestination().GetIp(), strategy.GetDestination().GetPort()), nil
		}
	}
	return "", util.ErrorfWithPos("no match rules, expect: %v, actual: %v", strategy, tag)
}

func (c *Consumer) getTargetByConfig(hostName string) (string, error) {
	routing := c.config.Load()
	if _, ok := routing.GetHosts()[hostName]; !ok {
		return "", util.ErrorfWithPos("no such host [%v] in routing table", hostName)
	}
	host := routing.GetHosts()[hostName]

	var targetAddr string
	switch host.GetRoutingRule() {
	case routingpb.LoadBalancer_consistent_hash:
	case routingpb.LoadBalancer_random:
		targetAddr = c.randomRouting(lo.MapToSlice(host.GetEndpoints(), func(eid int64, endpoint *routingpb.Endpoint) string {
			return combineAddr(endpoint.GetIp(), endpoint.GetIp())
		}))
	case routingpb.LoadBalancer_weight:
	case routingpb.LoadBalancer_target:
	}

	return targetAddr, nil
}

func (c *Consumer) randomRouting(endpoints []string) string {
	index := random.Intn(len(endpoints))
	return endpoints[index]
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
