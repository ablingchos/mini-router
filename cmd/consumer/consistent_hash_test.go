package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"testing"
	"time"

	consumer "git.woa.com/kefuai/mini-router/consumer/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
)

func TestConsistentHash(t *testing.T) {
	configPath := "/data/home/kefuai/code_repository/mini-router/consumer/impl/config.yaml"
	host1 := "test1"
	virtualNode := 3

	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic: %v", r)
			debugInfo := debug.Stack()
			os.WriteFile("./panic.log", debugInfo, 0644)
			fmt.Println(err)
		}
	}()
	// level := zap.NewAtomicLevelAt(zapcore.Level(0))
	// l, err := mlog.New(level)
	// if err != nil {
	// 	mlog.Errorf("Fail", zap.Error(err))
	// }
	// mlog.SetL(l)
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

	ticker := time.NewTicker(3 * time.Second)
	i := 1
	for range ticker.C {
		resp1, err := sdk.GetTargetEndpoint(host1, "eid-"+strconv.Itoa(i))
		if err != nil {
			mlog.Errorf("failed to get target endpint of host [%v], err: %v", host1, err)
		}
		if i == 5 {
			i = 1
		} else {
			i++
		}
		mlog.Infof("target endpoint: %v", resp1)
	}
}
