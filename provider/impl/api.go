package provider

import (
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
)

// 获取sdk实例
func NewProvider(configPath string) (*Provider, error) {
	var err error
	once.Do(func() {
		provider = &Provider{}
		err = provider.initializeProvider(configPath)
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return provider, nil
}

// 测试用
func NewproviderForTest(port string, eid int64) (*Provider, error) {
	p := &Provider{}
	if err := p.initializeProviderForTest(port, eid); err != nil {
		mlog.Fatalf("failed to initial provider: %v", err)
	}
	return p, nil
}

// 向控制面注册并运行sdk
func (p *Provider) Run() error {
	return p.register()
}

// graceful stop
func (p *Provider) Stop() {
	p.cancel()
}
