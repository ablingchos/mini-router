package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"testing"
	"time"

	consumer "git.woa.com/kefuai/mini-router/consumer/impl"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
)

func TestKeyRouting(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic: %v", r)
			debugInfo := debug.Stack()
			os.WriteFile("./panic.log", debugInfo, 0644)
			fmt.Println(err)
		}
	}()
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

	// 完全匹配"eid-1"的key，目的地址为9.134.78.221:10005
	sdk.SetRule(host1, &routingpb.UserRule{
		MatchRule: &routingpb.MatchRule{
			Match:   1,
			Content: "eid-1",
		},
		Destination: &routingpb.Endpoint{
			Ip:   "9.134.78.221",
			Port: "10005",
		},
	}, time.Second)

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
