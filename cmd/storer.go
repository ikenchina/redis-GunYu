package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/mgtv-tech/redis-GunYu/config"
	"github.com/mgtv-tech/redis-GunYu/pkg/filter"
	"github.com/mgtv-tech/redis-GunYu/pkg/log"
	"github.com/mgtv-tech/redis-GunYu/pkg/rdb"
	"github.com/mgtv-tech/redis-GunYu/pkg/redis"
	"github.com/mgtv-tech/redis-GunYu/pkg/redis/client"
	"github.com/mgtv-tech/redis-GunYu/pkg/store"
	"github.com/mgtv-tech/redis-GunYu/pkg/sync"
	"github.com/mgtv-tech/redis-GunYu/pkg/util"
)

type StorerCmd struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger log.Logger
}

func NewStorerCmd() *StorerCmd {
	ctx, c := context.WithCancel(context.Background())
	return &StorerCmd{
		ctx:    ctx,
		cancel: c,
		logger: log.WithLogger(""),
	}
}

func (sc *StorerCmd) Name() string {
	return "redis.storer"
}

func (sc *StorerCmd) Stop() error {
	sc.cancel()
	return nil
}

func (rc *StorerCmd) Run() error {
	dir := config.GetFlag().StorerCmd.Dir

	storer := store.NewStoreReader(dir)
	reader := storer.GetRdbReader()

	var filterKey *filter.RedisCmdFilter
	if len(config.GetFlag().StorerCmd.Filter.Keys) > 0 {
		filterKey = &filter.RedisCmdFilter{}
		filterKey.InsertPrefixKeyWhiteList(config.GetFlag().StorerCmd.Filter.Keys)
		filterKey.InsertCmdBlackList([]string{"select", "ping"}, true)
	}

	filterDb := config.GetFlag().StorerCmd.Filter.Db
	waiter := sync.NewWaitCloser(nil)
	if reader != nil {
		fmt.Println("-------- scan rdb -----------")

		reader.Start(waiter)

		ioReader := reader.IoReader()

		var readBytes atomic.Int64

		pipe := redis.ParseRdb(ioReader, &readBytes, config.RDBPipeSize, "")

		var e *rdb.BinEntry
		var ok bool
		curDb := -1
		for {
			select {
			case e, ok = <-pipe:
				if !ok {
					return nil
				}
				if e.Err != nil { // @TODO corrupted data
					panic(e.Err)
				}
				if e.Done {
					goto AOF
				}
			case <-waiter.Context().Done():
				return waiter.Error()
			}

			// filter
			if filterDb != -1 && e.DB != filterDb {
				continue
			}

			matched := false
			if filterKey != nil {
				if !filterKey.FilterKey(util.BytesToString(e.Key)) {
					matched = true
				}
			}

			if matched {
				if curDb != e.DB {
					fmt.Printf("------ DB(%d) --------\n", e.DB)
					curDb = e.DB
				}
				strs := e.PrintString()
				for _, str := range strs {
					fmt.Println(str)
				}
			}
		}

	}

AOF:

	reader = storer.GetAofReader()
	if reader != nil {
		fmt.Println("-------- scan aof -----------")
		reader.Start(waiter)

		for {
			decoder := client.NewDecoder(reader.IoReader())
			resp, _, err := client.MustDecodeOpt(decoder)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				panic(err)
			}

			sCmd, argv, err := client.ParseArgs(resp) // lower case
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				panic(err)
			}
			if filterKey.FilterCmd(sCmd) {
				continue
			}
			_, f := filterKey.FilterCmdKey(sCmd, argv)
			if !f {
				fmt.Printf("%s", sCmd)
				for _, arg := range argv {
					fmt.Printf(" %s", arg)
				}
				fmt.Println()
			}
		}
		fmt.Println(" ------------ completed ----------- ")

	}

	return nil

}
