package random

import (
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ScalableRand 可水平扩展的随机数发生器
type ScalableRand struct {
	initSeed int64
	randPool *sync.Pool
}

// NewScalableRand 初始化随机数发生器
func NewScalableRand() *ScalableRand {
	scalableRand := &ScalableRand{
		randPool: &sync.Pool{},
	}
	cpuCount := runtime.NumCPU()
	for i := 0; i < cpuCount; i++ {
		scalableRand.randPool.Put(rand.New(rand.NewSource(scalableRand.getRandSeed())))
	}
	return scalableRand
}

// getRandSeed 循环并获取唯一的随机数种子
func (s *ScalableRand) getRandSeed() int64 {
	var seed int64
	for {
		seed = time.Now().UnixNano()
		if s.getAndSetInitSeed(seed) {
			break
		}
		time.Sleep(1)
	}
	return seed
}

// getAndSetInitSeed 获取并比较种子数
func (s *ScalableRand) getAndSetInitSeed(seed int64) bool {
	initSeed := atomic.LoadInt64(&s.initSeed)
	if initSeed == seed {
		return false
	}
	return atomic.CompareAndSwapInt64(&s.initSeed, initSeed, seed)
}

// Intn 获取随机数
func (s *ScalableRand) Intn(n int) int {
	var randSeed *rand.Rand
	value := s.randPool.Get()
	if nil != value {
		randSeed = value.(*rand.Rand)
	} else {
		randSeed = rand.New(rand.NewSource(s.getRandSeed()))
	}
	randValue := randSeed.Intn(n)
	s.randPool.Put(randSeed)
	return randValue
}

// 全局随机种子
var globalRand *ScalableRand

// Intn 返回全局随机数
func Intn(n int) int {
	return globalRand.Intn(n)
}

// init 初始化全局随机种子
func init() {
	globalRand = NewScalableRand()
}
