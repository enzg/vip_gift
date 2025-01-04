// cmd/main.go
package main

import (
	"fmt"
	"log"

	"10000hk.com/vip_gift/config"
	"10000hk.com/vip_gift/internal/handler"
	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/service"
)

func main() {
	db := config.InitDB()
	esClient := config.InitES()
	app := config.SetupFiber()

	// 路由组： /api/product/gift
	api := app.Group("/api/product/gift")
	api.Use(handler.JWTMiddleware(config.JWTSecretKey))

	// Gnc
	gncRepo := repository.NewGncRepo(db)
	gncSvc := service.NewGncService(gncRepo)
	gncHdl := handler.NewGncHandler(gncSvc)
	gncHdl.RegisterRoutes(api)

	// Pub
	pubRepo := repository.NewPubRepo(db)
	pubSvc := service.NewPubService(pubRepo, esClient)
	pubHdl := handler.NewPubHandler(pubSvc)
	pubHdl.RegisterRoutes(api)

	addr := ":3001"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(app.Listen(addr))
}
