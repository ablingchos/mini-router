package main

import (
	consumer "git.woa.com/kefuai/mini-router/consumer/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	configPath  = "/data/home/kefuai/code_repository/mini-router/consumer/impl/config.yaml"
	host1       = "test1"
	virtualNode = 3
)

func main() {
	level := zap.NewAtomicLevelAt(zapcore.Level(0))
	l, err := mlog.New(level)
	if err != nil {
		mlog.Errorf("Fail", zap.Error(err))
	}
	mlog.SetL(l)
	sdk, err := consumer.NewConsumer(configPath, virtualNode)
	if err != nil {
		mlog.Errorf("failed to get a new consumer sdk: %v", err)
		return
	}

	if err := sdk.Init(); err != nil {
		mlog.Errorf("failed to init consumer: %v", err)
		return
	}

	endpoints, err := sdk.GetEndpoints(host1)
	if err != nil {
		mlog.Errorf("failed to get target endpints of host [%v]", host1)
		return
	}

	mlog.Info("successfully get all hosts", zap.Any("host", host1), zap.Any("number of endpoints", len(endpoints)))

	resp1, err := sdk.GetTargetEndpoints(host1, "")
	if err != nil {
		mlog.Errorf("failed to get target endpint of host [%v]", host1)
		return
	}
	mlog.Infof("target endpoint: %v", resp1)
}
