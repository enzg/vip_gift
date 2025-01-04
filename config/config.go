// config/config.go
package config

import (
	"log"

	// MySQL + GORM
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	// Elasticsearch
	"github.com/elastic/go-elasticsearch/v7"

	// 你的项目结构，包含数据表结构与其他类型
	"10000hk.com/vip_gift/internal/types"

	// Fiber & Middleware
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// InitDB 连接 MySQL 并自动迁移表
func InitDB() *gorm.DB {
	// TODO: 将 "#mysql_db url" 改成你的 DSN
	dsn := "#mysql_db url"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}

	// 自动迁移需要的表
	db.AutoMigrate(
		&types.GncEntity{},
		&types.PubEntity{},
		&types.PubComposeEntity{},
	)

	return db
}

// InitTestDB 专门给测试环境使用，建库、自动迁移等
func InitTestDB() *gorm.DB {
	// TODO: 将 "#mysql_db url" 改成你的测试数据库 DSN
	dsn := "#mysql_db url"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect test db: %v", err)
	}

	// 每次 DROP 再重建，仅供测试
	db.Migrator().DropTable(
		&types.PubComposeEntity{},
		&types.PubEntity{},
		&types.GncEntity{},
	)
	db.AutoMigrate(
		&types.GncEntity{},
		&types.PubEntity{},
		&types.PubComposeEntity{},
	)

	return db
}

// InitES 连接到 Elasticsearch
func InitES() *elasticsearch.Client {
	// TODO: 将 "#es_url" 替换为实际的 ES 地址, 例如 "http://localhost:9200"
	esURL := "#es_url"

	cfg := elasticsearch.Config{
		Addresses: []string{esURL},
		// 如果 ES 需要用户名密码, 在此加:
		// Username: "elastic",
		// Password: "changeme",
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Printf("failed to connect ES: %v", err)
		return nil
	}
	resp, err := client.Indices.Exists([]string{"products_index"})
	if err == nil && resp.StatusCode == 404 {
		// create index
	}

	// 可以在此处执行一些初始化操作，比如检查索引是否存在，若无则创建
	// （略）

	log.Println("Elasticsearch client initialized!")
	return client
}

// JWTSecretKey 需要与你的其他模块共享
var JWTSecretKey = "#uuid_for_jwt"

// SetupFiber 启动 Fiber，附带默认中间件
func SetupFiber() *fiber.App {
	app := fiber.New()
	// 使用默认配置
	app.Use(cors.New())

	// 这里可以添加更多全局中间件，如日志、异常恢复等
	// app.Use(logger.New())
	// app.Use(recover.New())

	return app
}
