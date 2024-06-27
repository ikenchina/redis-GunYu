package cluster

import (
	"context"
	"strings"
	gsync "sync"
	"time"

	"github.com/mgtv-tech/redis-GunYu/config"
	"github.com/mgtv-tech/redis-GunYu/pkg/log"
	"github.com/mgtv-tech/redis-GunYu/pkg/redis/client"
	"github.com/mgtv-tech/redis-GunYu/pkg/redis/client/common"
	"github.com/mgtv-tech/redis-GunYu/pkg/sync"
)

func NewRedisCluster(ctx context.Context, cfg config.RedisConfig, ttl int) (Cluster, error) {
	cli, err := client.NewRedis(cfg)
	if err != nil {
		return nil, err
	}

	ctx2, can2 := context.WithCancel(ctx)
	rc := &redisCluster{
		redisCli: cli,
		ttl:      ttl,
		ctx:      ctx2,
		cancel:   can2,
		wait:     gsync.WaitGroup{},
	}
	return rc, nil
}

type redisCluster struct {
	redisCli client.Redis
	ttl      int
	ctx      context.Context
	cancel   context.CancelFunc
	wait     gsync.WaitGroup
}

func (c *redisCluster) Close() error {
	c.cancel()
	c.wait.Wait()
	return c.redisCli.Close()
}

func (c *redisCluster) NewElection(ctx context.Context, prefix string) Election {
	return nil
}

func (c *redisCluster) Register(ctx context.Context, svcPath string, id string) error {
	err := common.StringIsOk(c.redisCli.Do("SET", svcPath+id, id, "EX", c.ttl))
	if err != nil {
		return err
	}

	c.wait.Add(1)

	// keepalive
	sync.SafeGo(func() {
		itv := time.Duration(c.ttl/4) * time.Second
		defer c.wait.Done()
		for {
			select {
			case <-time.After(itv):
				err := common.StringIsOk(c.redisCli.Do("SET", svcPath+id, id, "EX", c.ttl))
				if err != nil {
					log.Errorf("Register : id(%d), error(%v)", id, err)
				}
			case <-c.ctx.Done():
				c.redisCli.Do("DEL", svcPath+id)
				return
			}
		}
	}, nil)
	return nil
}

func (c *redisCluster) Discovery(ctx context.Context, svcPath string) ([]string, error) {
	bb := c.redisCli.NewBatcher()
	bb.Put("KEYS", svcPath+"*")
	replies, err := bb.Exec()
	if err != nil {
		return nil, err
	}

	ids := []string{}
	for _, res := range replies {
		srs, err := common.Strings(res, nil)
		if err != nil {
			return nil, err
		}
		for _, s := range srs {
			id := strings.TrimLeft(s, svcPath)
			if len(id) > 0 {
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}
