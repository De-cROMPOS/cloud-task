package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"main/balancer"
	"main/handlers"
	"main/internal"
	redisdb "main/redisDB"
)

func main() {
	// Initializing redis client
	redisClient := redisdb.NewClient("localhost:6379")
	if err := redisClient.Ping(context.Background()); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}

	// Initializing balancer
	lb := balancer.NewRoundRobin()
	cfg := balancer.NewConfigData()
	if err := cfg.GetCfgData(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := lb.UpdateServers(cfg); err != nil {
		log.Fatalf("Failed to update servers: %v", err)
	}

	// graceful shutdown ctx
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Background tickers
	go internal.RunConfigUpdater(ctx, lb, cfg)         // Cfg updater
	go redisClient.StartRefillTicker(ctx, time.Second) // Token refiller

	mux := http.NewServeMux()

	// Making handlers
	dbHandler := handlers.NewDBHandler(redisClient) // Redis handler 
	balancerHandler := handlers.NewBalancerHandler(redisClient, lb) // Balancer handler

	mux.Handle("/clients", dbHandler)
	mux.Handle("/", balancerHandler)

	// Server settings
	addr := ":"+lb.Port
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Starting server
	go func() {
		log.Println("Server started on", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Waiting for signals for graceful shutdown
	internal.HandleShutdown(server, cancel)
}
