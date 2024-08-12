package consumer

import (
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func NewConsumer(configPath string) (*Consumer, error) {
	var err error
	once.Do(func() {
		consumer = &Consumer{}
		err = consumer.initializeConsumer(configPath)
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return consumer, nil
}

func (c *Consumer) Init() error {
	config := c.getConfig()
	resp, err := c.discoverClient.ConsumerInit(c.ctx, &consumerpb.ConsumerInitRequest{
		GroupName: config.GetName(),
		HostName:  lo.Keys(config.GetHosts()),
	})
	if err != nil {
		return util.ErrorWithPos(err)
	}
	c.coverRoutingTable(resp.GetGroup(), resp.GetVersion())
	go c.watchLoop()
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
func (c *Consumer) GetTargetEndpoints(hostName string, tag string) (string, error) {
	if tag != "" {
		target, err := c.getTargetByTag(hostName, tag)
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
func (c *Consumer) SetRule(hostName string, rule *routingpb.UserRule, timoeout time.Duration) error {
	host := &routingpb.Host{
		Name:     hostName,
		UserRule: rule,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routingTable.GetHosts()[hostName] = host
	mlog.Info("set user rule successfully", zap.Any("user rule", host))
	return nil
}
