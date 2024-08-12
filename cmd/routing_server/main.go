package main

import (
	controller "git.woa.com/kefuai/mini-router/controller/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
)

func main() {
	server, err := controller.NewRoutingServer()
	if err != nil {
		mlog.Fatalf("failed to start routing server")
	}

	server.Run()
	select {}
}
