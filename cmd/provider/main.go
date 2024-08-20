package main

import (
	"strconv"
	"time"

	provider "git.woa.com/kefuai/mini-router/provider/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	configPath = "/data/home/kefuai/code_repository/mini-router/provider/impl/config1.yaml"
)

func main() {
	// sdk, err := provider.NewProvider(configPath)
	// if err != nil {
	// 	mlog.Errorf("failed to get a new provider sdk: %v", err)
	// 	return
	// }

	// if err := sdk.Run(); err != nil {
	// 	mlog.Errorf("failed to register provider: %v", err)
	// 	return
	// }
	level := zap.NewAtomicLevelAt(zapcore.Level(0))
	l, err := mlog.New(level)
	if err != nil {
		mlog.Errorf("Fail", zap.Error(err))
	}
	mlog.SetL(l)
	for i := 10011; i <= 10020; i++ {
		time.Sleep(50 * time.Millisecond)
		go func(i int) {
			sdk, err := provider.NewproviderForTest(strconv.Itoa(i), int64(i))
			if err != nil {
				mlog.Fatalf("failed to get a new provider sdk: %v", err)
			}
			if err := sdk.Run(); err != nil {
				mlog.Fatalf("failed to register provider: %v", err)
			}
		}(i)
	}
	mlog.Info("server register successfully")
	select {}
}
