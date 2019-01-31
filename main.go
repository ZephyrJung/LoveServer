package main

import (
	"github.com/ZephyrJung/LoveServer/server"
	"github.com/ZephyrJung/LoveServer/client"
)

func main() {
	go server.Start()
	go client.Start()
}
