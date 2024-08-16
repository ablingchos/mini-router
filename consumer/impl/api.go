package consumer

import (
	"sync/atomic"
	"time"

	consistenthash "git.woa.com/kefuai/mini-router/consumer/impl/algorithm/hash"
	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"go.uber.org/zap"
)

func NewConsumer(configPath string, virtualNode int) (*Consumer, error) {
	var err error
	once.Do(func() {
		consumer = &Consumer{
			hostName:    make([]string, 0),
			hashRing:    atomic.Pointer[map[string]*consistenthash.HashRing]{},
			virtualNode: virtualNode,
		}
		emptyMap := make(map[string]*consistenthash.HashRing)
		consumer.hashRing.Store(&emptyMap)
		err = consumer.initializeConsumer(configPath)
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return consumer, nil
}

func (c *Consumer) Init() error {
	resp, err := c.discoverClient.ConsumerInit(c.ctx, &consumerpb.ConsumerInitRequest{
		GroupName: c.groupName,
		HostName:  c.hostName,
	})
	if err != nil {
		return util.ErrorWithPos(err)
	}
	c.initRoutingTable(resp.GetGroup())
	go c.reportMetrics()
	return nil
}

func (c *Consumer) Stop() {
	c.cancel()
	mlog.Info("received consumer stop signal, start to shutdown")
}

// 获取目标host下所有可用的endpoint
func (c *Consumer) GetEndpoints(hostName string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	resp := []string{}
	if _, ok := c.routingTable.Hosts[hostName]; !ok {
		return nil, util.ErrorfWithPos("no such host in routing table")
	}
	host := c.routingTable.Hosts[hostName]
	for _, endpoint := range host.Endpoints {
		resp = append(resp, endpoint.GetIp()+":"+endpoint.GetPort())
	}

	return resp, nil
}

// 获取满足当前路由规则的一个endpoint
func (c *Consumer) GetTargetEndpoint(hostName string, key string) (string, error) {
	if key != "" {
		target, err := c.getTargetByKey(hostName, key)
		if err != nil {
			return "", util.ErrorWithPos(err)
		}
		return target, nil
	}

	target, err := c.getTargetByConfig(hostName)
	if err != nil {
		return "", util.ErrorWithPos(err)
	}

	return target, nil
}

// 设置路由规则
func (c *Consumer) SetRule(hostName string, rule *routingpb.UserRule, timoeout time.Duration) {
	host := &routingpb.Host{
		Name:     hostName,
		UserRule: rule,
	}
	config := c.config.Load()
	config.GetHosts()[hostName] = host
	c.config.Store(config)

	mlog.Info("set user rule successfully", zap.Any("user rule", host))
}
