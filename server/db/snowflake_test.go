package db

import (
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
)

func TestSnowflake(t *testing.T) {
	var now = time.Now()

	id := NewZeroID(now)

	s := snowflake.ParseInt64(id).Time()
	m := now.UnixNano() / int64(time.Millisecond)

	if s != m {
		t.Fatalf("Unequal time: %d != %d", s, m)
	}
}
