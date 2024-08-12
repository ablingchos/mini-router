package main

import (
	controller "git.woa.com/kefuai/mini-router/controller/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
)

func main() {
	server, err := controller.NewRoutingWatcher()
	if err != nil {
		mlog.Fatal("failed to start routing watcher")
	}

	mlog.Debugf("get server successfully")
	server.Run()
	select {}
}
