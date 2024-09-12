package client

import (
	"bufio"

	"github.com/mgtv-tech/redis-GunYu/config"
	"github.com/mgtv-tech/redis-GunYu/pkg/redis/client/common"
	"github.com/mgtv-tech/redis-GunYu/pkg/redis/client/conn"
)

// Redis interface
type Redis interface {
	Close() error
	Do(string, ...interface{}) (interface{}, error)
	Send(string, ...interface{}) error
	SendAndFlush(string, ...interface{}) error
	Receive() (interface{}, error)
	ReceiveString() (string, error)
	ReceiveBool() (bool, error)
	BufioReader() *bufio.Reader
	BufioWriter() *bufio.Writer
	Flush() error
	RedisType() config.RedisType
	Addresses() []string

	NewBatcher() common.CmdBatcher

	// for cluster
	IterateNodes(result func(string, interface{}, error), cmd string, args ...interface{})
	ClusterMultiDb() bool
}

func NewRedis(cfg config.RedisConfig) (Redis, error) {
	if cfg.IsCluster() {
		return NewRedisCluster(cfg)
	}
	return conn.NewRedisConn(cfg)
}
