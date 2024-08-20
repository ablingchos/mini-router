package main

import (
	"fmt"
	"os"
	"runtime/debug"

	controller "git.woa.com/kefuai/mini-router/controller/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic: %v", r)
			debugInfo := debug.Stack()
			os.WriteFile("./panic.log", debugInfo, 0644)
			fmt.Println(err)
		}
	}()
	level := zap.NewAtomicLevelAt(zapcore.Level(0))
	l, err := mlog.New(level)
	if err != nil {
		mlog.Errorf("Fail", zap.Error(err))
	}
	mlog.SetL(l)
	server, err := controller.NewRegisterServer()
	if err != nil {
		mlog.Fatal("failed to start register server")
	}

	go server.Run()
	select {}
}
