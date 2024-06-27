package cluster

import (
	"context"
	"errors"

	"github.com/mgtv-tech/redis-GunYu/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type etcdCluster struct {
	cli  *clientv3.Client
	sess *concurrency.Session
}

func NewEtcdCluster(ctx context.Context, cfg config.EtcdConfig) (Cluster, error) {
	cli, err := clientv3.New(clientv3.Config{
		Context:              ctx,
		Endpoints:            cfg.Endpoints,
		AutoSyncInterval:     cfg.AutoSyncInterval,
		DialTimeout:          cfg.DialTimeout,
		DialKeepAliveTime:    cfg.DialKeepAliveTime,
		DialKeepAliveTimeout: cfg.DialKeepAliveTimeout,
		Username:             cfg.Username,
		Password:             cfg.Password,
		RejectOldCluster:     cfg.RejectOldCluster,
	})
	if err != nil {
		return nil, err
	}
	sess, err := concurrency.NewSession(cli, concurrency.WithTTL(cfg.Ttl))
	if err != nil {
		cli.Close()
		return nil, err
	}
	return &etcdCluster{
		cli:  cli,
		sess: sess,
	}, nil
}

func (c *etcdCluster) Close() error {
	if c.sess != nil {
		err := c.sess.Close()
		return errors.Join(err, c.cli.Close())
	}

	return nil
}

func (c *etcdCluster) NewElection(ctx context.Context, prefix string) Election {
	return &etcdElection{
		cli:       c.cli,
		keyPrefix: prefix,
		sess:      c.sess,
	}
}

func (c *etcdCluster) Register(ctx context.Context, svcPath string, id string) error {
	_, err := c.cli.Put(ctx, svcPath+id, id, clientv3.WithLease(c.sess.Lease()))
	return err
}

func (c *etcdCluster) Discovery(ctx context.Context, svcPath string) ([]string, error) {
	resp, err := c.cli.Get(ctx, svcPath, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, resp.Count)
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Value))
	}

	return keys, nil
}
