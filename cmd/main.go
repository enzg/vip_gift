// cmd/main.go
package main

import (
	"fmt"
	"log"
	"os"

	"10000hk.com/vip_gift/config"
	"10000hk.com/vip_gift/internal/handler"
	"10000hk.com/vip_gift/internal/repository"
	"10000hk.com/vip_gift/internal/service"
)

func main() {
	config.LoadEnv()
	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		log.Fatal("jwtSecretKey environment variable is not set")
	}
	db := config.InitDB()
	esClient := config.InitES()
	app := config.SetupFiber()

	// 路由组： /api/product/gift
	api := app.Group("/api/product/gift")
	api.Use(handler.JWTMiddleware(jwtSecretKey))
	// 路由组： /api/product/gift/shop (不需要 JWT)
	noAuthApi := app.Group("/api/product/gift/shop")

	// Gnc
	gncRepo := repository.NewGncRepo(db)
	gncSvc := service.NewGncService(gncRepo)
	gncHdl := handler.NewGncHandler(gncSvc)
	gncHdl.RegisterRoutes(api)

	// Pub
	pubRepo := repository.NewPubRepo(db)
	pubSvc := service.NewPubService(pubRepo, esClient)
	pubHdl := handler.NewPubHandler(pubSvc)
	pubHdl.RegisterRoutes(api, noAuthApi)

	addr := ":3001"
	fmt.Printf("Server listening on %s\n", addr)
	log.Fatal(app.Listen(addr))
}
