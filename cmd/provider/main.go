package main

import (
	provider "git.woa.com/kefuai/mini-router/provider/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
)

const (
	configPath = "/data/home/kefuai/code_repository/mini-router/provider/impl/config1.yaml"
)

func main() {
	sdk, err := provider.NewProvider(configPath)
	if err != nil {
		mlog.Errorf("failed to get a new provider sdk: %v", err)
		return
	}

	if err := sdk.Run(); err != nil {
		mlog.Errorf("failed to register provider: %v", err)
		return
	}

	select {}
}
