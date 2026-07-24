package main

import (
	"log"

	"github.com/ZephyrJung/LoveServer/internal/config"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	_ = cfg
	log.Println("LoveServer starting...")
}