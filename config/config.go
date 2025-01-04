// config/config.go
package config

import (
	"log"

	"10000hk.com/vip_gift/internal/types"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	dsn := "#mysql_db url"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	// 自动迁移
	db.AutoMigrate(
		&types.GncEntity{},
		&types.PubEntity{},
		&types.PubComposeEntity{},
	)
	return db
}

func InitTestDB() *gorm.DB {
	dsn := "#mysql_db url"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect test db: %v", err)
	}
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

func SetupFiber() *fiber.App {
	app := fiber.New()
	// 使用默认配置
	app.Use(cors.New())
	// 这里如果需要全局中间件（如日志、恢复）可加
	return app
}
