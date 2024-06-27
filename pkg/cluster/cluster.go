package cluster

import (
	"context"
	"errors"
)

var (
	ErrNoLeader  = errors.New("no leader")
	ErrNotLeader = errors.New("not a leader")
)

type ClusterRole int

const (
	RoleCandidate ClusterRole = iota
	RoleFollower  ClusterRole = iota
	RoleLeader    ClusterRole = iota
)

type RoleInfo struct {
	Address string
	Role    ClusterRole
}

type Cluster interface {
	Close() error
	NewElection(ctx context.Context, prefix string) Election
	Register(ctx context.Context, svcPath string, id string) error
	Discovery(ctx context.Context, svcPath string) ([]string, error)
}

type Election interface {
	Renew(ctx context.Context) error
	Leader(ctx context.Context) (*RoleInfo, error)
	Campaign(ctx context.Context, val string) (ClusterRole, error)
	Resign(ctx context.Context) error
}
