package main

import (
	"testing"
	"time"

	consumer "git.woa.com/kefuai/mini-router/consumer/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
)

func TestBench(t *testing.T) {
	configPath := "/data/home/kefuai/code_repository/mini-router/consumer/impl/config.yaml"
	virtualNode := 3

	for {
		sdk, err := consumer.NewConsumerForTest(configPath, virtualNode)
		if err != nil {
			mlog.Errorf("failed to get a new consumer sdk: %v", err)
			continue
		}

		if err := sdk.InitForTest(); err != nil {
			mlog.Errorf("failed to init consumer: %v", err)
			continue
		}

		time.Sleep(1 * time.Second)
		sdk.Stop()
	}
}
