package store

import (
	"bufio"

	"github.com/mgtv-tech/redis-GunYu/pkg/io/pipe"
	"github.com/mgtv-tech/redis-GunYu/pkg/log"
)

type StoreReader struct {
	dir     string
	logger  log.Logger
	dataSet *dataSet
}

func NewStoreReader(dir string) *StoreReader {
	logger := log.WithLogger("")

	set := initDataSet(logger, dir)

	return &StoreReader{
		dir:     dir,
		logger:  logger,
		dataSet: set,
	}
}

func (sr *StoreReader) GetRdbReader() *Reader {
	rdb := sr.dataSet.GetRdb()
	if rdb == nil {
		return nil
	}

	piper, pipew := pipe.NewSize(2 * 1024 * 1024)
	reader := bufio.NewReaderSize(piper, 2*1024*1024)

	rd := &Reader{}
	rr, err := NewRdbReader(pipew, sr.dir, rdb.left, rdb.Size(), false)
	if err != nil {
		panic(err)
	}
	rd.rdb = rr
	rd.reader = reader
	rd.size = rdb.Size()
	rd.left = rdb.left
	rd.logger = sr.logger

	return rd
}

func (sr *StoreReader) GetAofReader() *Reader {
	if len(sr.dataSet.aofSegs) == 0 {
		return nil
	}

	aof1 := sr.dataSet.aofSegs[0]

	piper, pipew := pipe.NewSize(2 * 1024 * 1024)
	reader := bufio.NewReaderSize(piper, 2*1024*1024)

	rd := &Reader{}

	rr, err := NewAofRotateReader(sr.dir, aof1.Left(), sr, pipew, false, true)
	if err != nil {
		panic(err)
	}

	rd.left = aof1.Left()
	rd.aof = rr
	rd.reader = reader
	rd.size = -1
	rd.logger = log.WithLogger("")

	return rd

}

func (sr *StoreReader) hasWriter(left int64) bool {
	return false
}

func (sr *StoreReader) lastSeg() int64 {
	return sr.dataSet.LastAofSeg()
}
