package id

import (
	"github.com/bwmarrin/snowflake"
	"math/rand"
	"myproxy/internal/mlog"
	"sync"
)

var (
	once sync.Once
	node *snowflake.Node
)

func GetSnowflakeID() snowflake.ID {
	once.Do(func() {
		nod, err := snowflake.NewNode(rand.Int63n(1023))
		if err != nil {
			mlog.Fatal("failed to create snowflake node: " + err.Error())
		}
		node = nod
	})
	return node.Generate()
}
