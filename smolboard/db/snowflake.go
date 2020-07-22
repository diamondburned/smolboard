package db

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

const (
	postIDNode int64 = iota
	sessionIDNode
)

var (
	postIDGen    = mustSnowflake(postIDNode)
	sessionIDGen = mustSnowflake(sessionIDNode)
)

func mustSnowflake(node int64) *snowflake.Node {
	n, err := snowflake.NewNode(node)
	if err != nil {
		panic(err)
	}

	return n
}

func NewZeroID(t time.Time) int64 {
	epoch := t.UnixNano() / int64(time.Millisecond)
	epoch -= snowflake.Epoch

	return epoch << 22
}
