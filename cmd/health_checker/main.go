package main

import (
	controller "git.woa.com/kefuai/mini-router/controller/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
)

func main() {
	server, err := controller.NewRegisterServer()
	if err != nil {
		mlog.Fatal("failed to start register server")
	}

	go server.Run()
	select {}
}
