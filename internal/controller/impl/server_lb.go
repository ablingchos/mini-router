package controller

import (
	"sync/atomic"
)

type Balancer struct {
	counter atomic.Int32
}

func (b *Balancer) GetNextServer() string {
	index := b.counter.Add(1)
	if index < 0 {
		index = -index
	}
	httpGateway.mu.RLock()
	targetAddr := httpGateway.serverAddrs[index%int32(len(httpGateway.serverAddrs))]
	httpGateway.mu.Unlock()

	return targetAddr
}
