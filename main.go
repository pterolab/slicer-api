package main

import (
	"fmt"
	"log"
	"os"
	"slicer-api/servers"
	"strconv"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	mode := os.Getenv("SERVER_MODE")
	if mode == "" {
		mode = "http"
	}

	port, _ := strconv.ParseInt(os.Getenv("PORT"), 10, 0)
	addr := fmt.Sprintf("localhost:%d", port)

	switch mode {
	case "grpc":
		if err := servers.RunGRPC(addr); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	case "http":
		if err := servers.RunHTTP(addr); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	default:
		log.Fatalf("invalid SERVER_MODE: %q (allowed: \"grpc\" or \"http\")", mode)
	}
}
