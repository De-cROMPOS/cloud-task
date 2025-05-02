package internal

import (
	"context"
	"log"
	"main/balancer"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Available servers updater
func RunConfigUpdater(ctx context.Context, lb *balancer.RoundRobin, cfg *balancer.ConfigData) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := cfg.GetCfgData(); err != nil {
				log.Printf("Config reload error: %v", err)
				continue
			}

			if err := lb.UpdateServers(cfg); err != nil {
				log.Printf("Servers update error: %v", err)
			} else {
				log.Printf("Config reloaded successfully.\nAvailable backends: %v", cfg.Servers)
			}

		case <-ctx.Done():
			log.Println("Stopping config updater")
			return
		}
	}
}

// Graceful shutdown
func HandleShutdown(server *http.Server, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down server...")

	cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Server stopped")
}
