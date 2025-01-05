package pkg

import (
	"log"

	"github.com/bwmarrin/snowflake"
)

// InitSnowflake 生成一个返回闭包的函数,可在Service中注入
func InitSnowflake(nodeID int64) func() string {
    node, err := snowflake.NewNode(nodeID)
    if err != nil {
        log.Fatalf("init snowflake node error: %v\n", err)
    }
    log.Printf("Snowflake node init success with nodeID=%d\n", nodeID)

    return func() string {
        return node.Generate().String()
    }
}
