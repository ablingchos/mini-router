package provider

import (
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

// 被调方注册
func (p *Provider) Run() error {
	return p.register()
}

// graceful stop
func (p *Provider) Stop() {
	p.cancel()
}
