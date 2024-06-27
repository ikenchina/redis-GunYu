package cluster

import (
	"context"
	"testing"

	"github.com/mgtv-tech/redis-GunYu/config"
	"github.com/mgtv-tech/redis-GunYu/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestRegistration(t *testing.T) {

	cfgs := []config.RedisConfig{
		// {
		// 	Addresses: []string{"localhost:16300"},
		// 	Type:      config.RedisTypeCluster,
		// },
		{
			Addresses: []string{"localhost:6379"},
			Type:      config.RedisTypeStandalone,
		},
	}

	for _, temp := range cfgs {
		cfg := temp

		t.Run("", func(t *testing.T) {
			ctx := context.Background()
			rc1, err := NewRedisCluster(ctx, cfg, 10)
			assert.Nil(t, err)

			rc2, err := NewRedisCluster(ctx, cfg, 10)
			assert.Nil(t, err)

			svcs := []string{"a1", "a2", "b1"}
			prefix := "/reg/"

			// registration
			assert.Nil(t, rc1.Register(context.Background(), prefix, svcs[0]))
			assert.Nil(t, rc2.Register(context.Background(), prefix, svcs[1]))
			assert.Nil(t, rc2.Register(context.Background(), prefix, svcs[2]))

			// discovery
			dis, err := rc2.Discovery(context.Background(), prefix)
			assert.Nil(t, err)
			ids := util.SliceToMap[string](dis)

			for _, svc := range svcs {
				_, ok := ids[svc]
				assert.True(t, ok)
			}

			//  unregistration
			rc1.Close()
			rc2.Close()

			// discovery
			rc3, err := NewRedisCluster(ctx, cfg, 10)
			assert.Nil(t, err)

			dis, err = rc3.Discovery(context.Background(), prefix)
			assert.Nil(t, err)
			ids = util.SliceToMap[string](dis)

			for _, svc := range svcs {
				_, ok := ids[svc]
				assert.False(t, ok)
			}
			rc3.Close()
		})
	}

}
