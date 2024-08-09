package provider

import (
	"net/http"
	"sync"
)

type Provider struct {
	ControllerDomain string
	Group            string
	Host             string
	Ip               string
	Port             string
	Timeout          int64
	eid              *Eid
	httpClient       *http.Client
	stopCh           chan struct{}
}

type Eid struct {
	Eid int64 `json:"eid"`
}

var (
	once     sync.Once
	provider *Provider
)

// 获取sdk实例
func GetProvider(temp *Provider) *Provider {
	if provider == nil {
		provider = temp
	}
	return provider
}

// 被调方注册
func (p *Provider) Register(configPath string) error {
	once.Do(func() {
		p.httpClient = http.DefaultClient
		p.stopCh = make(chan struct{})
	})
	return p.register(configPath)
}

// graceful stop
func (p *Provider) Stop() {
	p.stopCh <- struct{}{}
}
