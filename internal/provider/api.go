package provider

import (
	"context"
	"os"
	"sync"

	"git.woa.com/kefuai/mini-router/internal/common"
	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v2"
)

const (
	healthCheckAddr = "localhost:5100"
)

type ProviderConfig providerpb.RegisterRequest

type Provider struct {
	ControllerDomain string
	config           *ProviderConfig
	eid              int64
	leaseId          int64
	ctx              context.Context
	cancel           context.CancelFunc

	discoverClient providerpb.ProviderServiceClient
}

type Eid struct {
	Eid int64 `json:"eid"`
}

var (
	once     sync.Once
	provider *Provider
)

// 获取sdk实例
func NewProvider(configPath string) (*Provider, error) {
	var err error
	once.Do(func() {
		err = provider.initializeProvider(configPath)
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return provider, nil
}

func (p *Provider) initializeProvider(configPath string) error {
	// 读配置
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	req := &providerpb.RegisterRequest{}
	err = yaml.Unmarshal(configBytes, &req)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.config = (*ProviderConfig)(req)
	// 获取ip
	ip, err := common.GetIpAddr()
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.config.Ip = ip

	provider.ctx, provider.cancel = context.WithCancel(context.Background())
	if err := provider.grpcConnect(); err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

// 被调方注册
func (p *Provider) Register(configPath string) error {
	return p.register()
}

func (p *Provider) grpcConnect() error {
	conn, err := grpc.NewClient(healthCheckAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	p.discoverClient = providerpb.NewProviderServiceClient(conn)
	return nil
}

// graceful stop
func (p *Provider) Stop() {
	p.cancel()
}
