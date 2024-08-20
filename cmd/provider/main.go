package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	provider "git.woa.com/kefuai/mini-router/provider/impl"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	configPath    = "/data/home/kefuai/code_repository/mini-router/provider/impl/config1.yaml"
	MaxGoroutines = 100  // 最大并发 goroutine 数量
	NumJobs       = 2000 // 需要执行的任务数量
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

	jobs := make(chan int, NumJobs)

	// 创建一个 WaitGroup 以等待所有 worker 完成
	var wg sync.WaitGroup

	// 启动 worker
	for i := 0; i < MaxGoroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for job := range jobs {
				time.Sleep(5 * time.Millisecond)

				sdk, err := provider.NewproviderForTest(strconv.Itoa(job+10000), int64(job))
				if err != nil {
					fmt.Printf("failed to get a new provider sdk: %v\n", err)
					continue
				}
				if err := sdk.Run(); err != nil {
					fmt.Printf("failed to register provider: %v\n", err)
				}
			}
		}()
	}

	// 将任务添加到任务队列中
	for i := 1; i <= NumJobs; i++ {
		jobs <- i
	}

	// 关闭任务队列并等待所有 worker 完成
	close(jobs)
	wg.Wait()
	mlog.Info("server register successfully")
	select {}
}
