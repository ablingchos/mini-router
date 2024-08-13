package main

import (
	"strconv"
	"time"

	consumer "git.woa.com/kefuai/mini-router/consumer/impl"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
)

const (
	configPath  = "/data/home/kefuai/code_repository/mini-router/consumer/impl/config.yaml"
	host1       = "test1"
	virtualNode = 3
)

func main() {
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

	ticker := time.NewTicker(5 * time.Second)
	i := 1
	for range ticker.C {
		resp1, err := sdk.GetTargetEndpoint(host1, "eid-"+strconv.Itoa(i))
		if err != nil {
			mlog.Errorf("failed to get target endpint of host [%v]", host1)
			return
		}
		// if i == 5 {
		// 	i = 1
		// } else {
		// 	i++
		// }
		mlog.Infof("target endpoint: %v", resp1)
	}

}
