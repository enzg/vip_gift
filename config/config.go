// config/config.go
package config

import (
	"context"
	"log"
	"os"
	"strings"

	// MySQL + GORM
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	// Elasticsearch
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/joho/godotenv"

	// 你的项目结构，包含数据表结构与其他类型
	"10000hk.com/vip_gift/internal/types"
	// Fiber & Middleware
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// LoadEnv 可选：在本地开发环境自动加载 .env
// 生产环境中可以用 docker 环境变量或其他方式
func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error loading .env, using system environment variables.")
	}
}

// InitDB 连接 MySQL 并自动迁移表
func InitDB() *gorm.DB {
	// TODO: 将 "#mysql_db url" 改成你的 DSN
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable is not set")
	}
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
	dsn := os.Getenv("DB_TEST_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable is not set")
	}
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
func InitES() *elasticsearch.Client {
	esURL := "http://localhost:9200"
	cfg := elasticsearch.Config{
		Addresses: []string{esURL},
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Printf("failed to create ES client: %v", err)
		return nil
	}

	indexName := "vip_pub"
	resp, err := client.Indices.Exists([]string{indexName})
	if err != nil {
		log.Printf("failed to check if index exists: %v", err)
		return nil
	}
	defer resp.Body.Close()

	// 如果索引不存在(返回404), 我们从外部 JSON 文件中读取 Mapping 并创建索引
	if resp.StatusCode == 404 {
		// 假设你的 JSON 文件叫做 "vip_pub_mapping.json" 并放在同级目录
		mappingBytes, err := os.ReadFile("assets/vip_pub_mapping.json")
		if err != nil {
			log.Printf("failed to read mapping file: %v", err)
			return client // 返回空或 client 看业务需求
		}

		// 将 JSON 内容转成字符串，然后提交到 ES
		req := esapi.IndicesCreateRequest{
			Index: indexName,
			Body:  strings.NewReader(string(mappingBytes)),
		}

		createResp, err := req.Do(context.Background(), client)
		if err != nil {
			log.Printf("failed to create index %s: %v", indexName, err)
			return client
		}
		defer createResp.Body.Close()

		if createResp.IsError() {
			log.Printf("Error creating index %s, status: %s", indexName, createResp.Status())
		} else {
			log.Printf("Index %s created successfully!", indexName)
		}
	} else {
		log.Printf("Index %s exists or check returned status: %d", indexName, resp.StatusCode)
	}

	log.Println("Elasticsearch client initialized!")
	return client
}
