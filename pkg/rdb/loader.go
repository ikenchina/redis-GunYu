package rdb

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"strconv"

	"github.com/mgtv-tech/redis-GunYu/config"
	"github.com/mgtv-tech/redis-GunYu/pkg/digest"
	"github.com/mgtv-tech/redis-GunYu/pkg/log"
	"github.com/mgtv-tech/redis-GunYu/pkg/util"
)

type Loader struct {
	*RdbReader
	crc                hash.Hash64
	db                 uint32
	lastEntry          *BinEntry
	logger             log.Logger
	targetRedisVersion string
	rdbVersion         int64
}

func NewLoader(r io.Reader, targetRedisVersion string) *Loader {
	l := &Loader{
		logger:             log.WithLogger(config.LogModuleName("[RdbLoader] ")),
		targetRedisVersion: targetRedisVersion,
	}
	l.crc = digest.New()
	l.RdbReader = NewRdbReader(io.TeeReader(r, l.crc))
	return l
}

// redis : rdb.c:rdbSaveRio
func (l *Loader) Header() error {
	header := make([]byte, 9)
	if err := l.readFull(header); err != nil {
		return err
	}
	if !bytes.Equal(header[:5], []byte("REDIS")) {
		return fmt.Errorf("header[:5] is not REDIS : %v", header[:5])
	}
	version, err := strconv.ParseInt(string(header[5:]), 10, 64)
	if err != nil {
		return fmt.Errorf("version is not integer : %v", header[5:])
	} else if version <= 0 || version > RdbVersion {
		return fmt.Errorf("unsupported version : %d", version)
	}
	l.rdbVersion = (version)
	return nil
}

func (l *Loader) Footer() error {
	crc1 := l.crc.Sum64()
	if crc2, err := l.ReadUint64(); err != nil {
		return err
	} else if crc2 == 0 {
		l.logger.Infof("no checksum")
	} else if crc1 != crc2 {
		return fmt.Errorf("checksum validation error : expect(%d), actual(%d)", crc2, crc1)
	}
	return nil
}

type BinEntry struct {
	DB           int
	Key          []byte
	Type         byte
	value        []byte
	ExpireAt     uint64
	IdleTime     uint32
	Freq         uint8
	Err          error
	ObjectParser Parser
	Done         bool
}

func (be *BinEntry) CanRestore() bool {
	if be.ObjectParser != nil {
		return be.ObjectParser.CanRestore()
	}
	return true
}

func (be *BinEntry) FirstBin() bool {
	if be.ObjectParser != nil {
		return be.ObjectParser.FirstBin()
	}
	return true
}

func (be *BinEntry) Value() []byte {
	if be.ObjectParser != nil {
		return be.ObjectParser.Value()
	}
	return be.value
}

func (be *BinEntry) DumpValue() []byte {
	if be.ObjectParser != nil {
		return be.ObjectParser.CreateValueDump()
	}
	return CreateValueDump(be.Type, be.value)
}

func (be *BinEntry) PrintString() []string {
	switch be.ObjectParser.Type() {
	case RdbObjectString:
		return []string{fmt.Sprintf("string : key(%s), value(%s), expireAt(%d)", be.Key, be.ObjectParser.Value(), be.ExpireAt)}
	case RdbObjectList:
		buf := bytes.NewBuffer(make([]byte, 0))
		buf.WriteString(fmt.Sprintf("list : key(%s) : ", be.Key))
		be.ObjectParser.ExecCmd(func(cmd string, args ...interface{}) error {
			if len(args) == 2 {
				buf.WriteString(fmt.Sprintf("%s, ", args[1].([]byte)))
			}
			return nil
		})
		return []string{buf.String()}
	case RdbObjectSet:
	case RdbObjectZSet:
		buf := bytes.NewBuffer(make([]byte, 0))
		buf.WriteString(fmt.Sprintf("zset : key(%s) : ", be.Key))
		be.ObjectParser.ExecCmd(func(cmd string, args ...interface{}) error {
			if len(args) == 3 {
				buf.WriteString(fmt.Sprintf("[%s, %s], ", args[1].([]byte), args[2].([]byte)))
			}
			return nil
		})
		return []string{buf.String()}
	case RdbObjectHash:
	case RdbObjectStream:
	case RdbObjectModule:
	case RdbObjectFunction:
	case RdbObjectAux:
	}
	return nil
}

func (l *Loader) Next() (entry *BinEntry, err error) {
	defer util.Xrecover(&err)

	entry = &BinEntry{
		DB: -1,
	}
	for {
		var t byte
		if l.totalEntries-l.readEntries == 0 {
			rtype := l.ReadByteP()
			t = rtype
		} else {
			t = l.lastEntry.Type
		}
		entry.Type = t
		switch t {
		case RdbFlagAUX:
			parser := l.newParser(t, l)
			entry.ObjectParser = parser
			entry.Key = parser.Key()
			entry.DB = int(l.db)
			return entry, nil
		case RdbFlagResizeDB:
			db_size := l.ReadLengthP()
			expire_size := l.ReadLengthP()
			l.logger.Debugf("RdbFlagResizeDB : dbsize(%d), expiresize(%d)", db_size, expire_size)
		case RdbFlagExpiryMS:
			ttlms := l.ReadUint64P()
			entry.ExpireAt = ttlms
		case RdbFlagExpiry:
			ttls := l.ReadUint32P()
			entry.ExpireAt = uint64(ttls) * 1000
		case RdbFlagSelectDB:
			dbnum := l.ReadLengthP()
			l.db = dbnum
		case RdbFlagEOF:
			return nil, nil // means EOF
		case RdbFlagModuleAux:
			_ = l.ReadLength64P() // uint64_t moduleid = rdbLoadLen(rdb,NULL);
			rdbLoadCheckModuleValue(l)
		case RdbFlagIdle:
			idle := l.ReadLengthP()
			entry.IdleTime = idle
		case RdbFlagFreq:
			freq := l.ReadUint8P()
			entry.Freq = freq
		case RdbTypeFunction2: //function
			parser := l.newParser(t, l)
			entry.ObjectParser = parser
			entry.Key = parser.Key()
			return entry, nil
		default:
			parser := l.newParser(t, l)
			entry.ObjectParser = parser
			entry.DB = int(l.db)
			entry.Key = parser.Key()
			l.lastEntry = entry
			return entry, nil
		}
	}
}

func rdbLoadCheckModuleValue(l *Loader) {
	var whenOpCode uint32
	for {
		whenOpCode = l.ReadLengthP() // int when_opcode = rdbLoadLen(rdb,NULL);
		_ = l.ReadLengthP()          // when
		if whenOpCode == rdbModuleOpcodeEof {
			break
		}

		switch whenOpCode {
		case rdbModuleOpcodeSint:
			fallthrough
		case rdbModuleOpcodeUint:
			_ = l.ReadLengthP()
		case rdbModuleOpcodeString:
			l.ReadStringP()
		case rdbModuleOpcodeFloat:
			l.ReadFloatP()
		case rdbModuleOpcodeDouble:
			l.ReadDoubleP()
		}
	}
}
