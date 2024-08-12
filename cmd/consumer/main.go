package main

import (
	consumer "git.woa.com/kefuai/mini-router/consumer/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
)

const (
	configPath = "/data/home/kefuai/code_repository/mini-router/consumer/impl/config.yaml"
	host       = "test1"
)

func main() {
	sdk, err := consumer.NewConsumer(configPath)
	if err != nil {
		mlog.Errorf("failed to get a new consumer sdk: %v", err)
		return
	}

	if err := sdk.Init(); err != nil {
		mlog.Errorf("failed to init consumer: %v", err)
		return
	}

	endpoints, err := sdk.GetEndpoints(host)
	if err != nil {
		mlog.Errorf("failed to get target endpints of host [%v]", host)
		return
	}

	mlog.Info("successfully get all hosts", zap.Any("host", host), zap.Any("endpoints", endpoints))
}
