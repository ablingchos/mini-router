package main

import (
	"fmt"
	"os"
	"runtime/debug"

	controller "git.woa.com/kefuai/mini-router/controller/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
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
	server, err := controller.NewRoutingWatcher()
	if err != nil {
		mlog.Fatal("failed to start routing watcher")
	}

	mlog.Debugf("get server successfully")
	server.Run()
	select {}
}
