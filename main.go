package main

import (
	"context"
	"log"

	"github.com/ZephyrJung/LoveServer/internal/config"
	"github.com/ZephyrJung/LoveServer/internal/server"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	srv := server.New(cfg)
	if err := srv.Init(ctx); err != nil {
		log.Fatalf("failed to init server: %v", err)
	}

	log.Fatal(srv.Start())
}