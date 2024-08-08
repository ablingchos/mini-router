package controller

import "sync/atomic"

type IDGenerator struct {
	Generator atomic.Int64
}
