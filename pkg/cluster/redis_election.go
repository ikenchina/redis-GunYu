package cluster

import (
	"context"

	"github.com/mgtv-tech/redis-GunYu/pkg/redis/client"
	"github.com/mgtv-tech/redis-GunYu/pkg/redis/client/common"
)

type redisElection struct {
	key string
	cli client.Redis
	ttl int
	id  string
}

func (e *redisElection) Renew(ctx context.Context) error {
	role, err := e.Campaign(ctx)
	if err != nil {
		return err
	}
	if role == RoleFollower {
		return ErrNotLeader
	}

	return nil
}

func (e *redisElection) Leader(ctx context.Context) (*RoleInfo, error) {
	res, err := common.String(e.cli.Do("GET", e.key))
	return &RoleInfo{
		Address: res,
		Role:    RoleLeader,
	}, err
}

func (e *redisElection) Campaign(ctx context.Context) (ClusterRole, error) {

	lua := `
local key = KEYS[1]
local value = ARGV[1]
local ttl = ARGV[2]

local currentValue = redis.call('GET', key)

if currentValue == false then
    redis.call('SET', key, value, 'EX', ttl)
    return 1
else
    if currentValue == value then
        redis.call('EXPIRE', key, ttl)
        return 1
    else
        return 0
    end
end
`

	ret, err := common.Int(e.cli.Do("eval", lua, []byte("1"), e.key, e.id, e.ttl))
	if err != nil {
		return RoleCandidate, err
	}

	if ret == 1 {
		return RoleLeader, nil
	}
	return RoleFollower, nil
}

func (e *redisElection) Resign(ctx context.Context) error {

	lua := `
local key = KEYS[1]
local value = ARGV[1]

local currentValue = redis.call('GET', key)

if currentValue == false then
    return 1
else
    if currentValue == value then
        redis.call('DEL', key)
        return 1
    else
        return 0
    end
end
`

	_, err := common.Int(e.cli.Do("eval", lua, []byte("1"), e.key, e.id, e.ttl))
	if err != nil {
		return err
	}

	return nil
}
