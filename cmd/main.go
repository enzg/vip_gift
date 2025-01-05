// cmd/main.go
package main

import (
	"fmt"
	"log"

	"10000hk.com/vip_gift/config"
	"10000hk.com/vip_gift/internal/handler"
	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/service"
	"10000hk.com/vip_gift/pkg"
)

func main() {
	config.LoadEnv()

	db := config.InitDB()
	esClient := config.InitES()
	app := config.SetupFiber()

	// 路由组： /api/product/gift
	api := app.Group("/api/product/gift")
	// api.Use(handler.JWTMiddleware(jwtSecretKey))

	// Pub
	pubRepo := repository.NewPubRepo(db)
	pubSvc := service.NewPubService(pubRepo, esClient)
	pubHdl := handler.NewPubHandler(pubSvc)
	pubHdl.RegisterRoutes(api)

	// Gnc
	gncRepo := repository.NewGncRepo(db)
	gncSvc := service.NewGncService(gncRepo)
	gncHdl := handler.NewGncHandler(gncSvc)
	gncHdl.RegisterRoutes(api)
	// Order
	orderRepo := repository.NewOrderRepo(db)
	// init kafkaWriter, snowflakeFn...
	//
	// 1) 初始化 Kafka & Snowflake from our new modules
	//    例如:
	kafkaWriter := pkg.InitKafkaWriter("localhost:9092", "order-create")
	snowflakeFn := pkg.InitSnowflake(1)
	orderSvc := service.NewOrderService(orderRepo, kafkaWriter, snowflakeFn)
	orderHdl := handler.NewOrderHandler(orderSvc)
	orderHdl.RegisterRoutes(api) // /orders

	addr := ":3001"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(app.Listen(addr))
}
