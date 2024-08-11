package consumer

import (
	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"github.com/samber/lo"
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
func (c *Consumer) GetEndpoints() {

}

// 获取满足当前路由规则的一个endpoint
func (c *Consumer) GetTargetEndpoints() {

}

// 设置路由规则
func (c *Consumer) SetRule() {

}
