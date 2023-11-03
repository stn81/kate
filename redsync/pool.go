package redsync

import (
	"github.com/redis/go-redis/v9"
	"github.com/stn81/kate/rdb"
)

// Pool maintains a pool of Redis connections.
type Pool interface {
	Get() redis.Cmdable
}

type defaultPool struct{}

func (p *defaultPool) Get() redis.Cmdable {
	return rdb.Get()
}
