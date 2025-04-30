package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"main/balancer"
)

func main() {
	// Initializing balancer
	lb := balancer.NewRoundRobin()

	// Loading cfg
	cfg := balancer.NewConfigData()
	if err := cfg.GetCfgData(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Updating server
	if err := lb.UpdateServers(cfg); err != nil {
		log.Fatalf("Failed to update servers: %v", err)
	}

	// Updating servers in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runConfigUpdater(ctx, lb, cfg)

	// Setting up reverse pproxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			server, err := lb.Generate()
			if err != nil {
				log.Printf("No available servers: %v", err)
				return
			}

			req.URL.Scheme = server.Addr.Scheme
			req.URL.Host = server.Addr.Host
			req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
			req.Host = server.Addr.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error: %v", err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	// setting up HTTP server
	server := &http.Server{
		Addr:    ":" + lb.Port,
		Handler: proxy,
	}

	// Graceful shutdown
	go handleShutdown(server, cancel)

	// Starting our balancer server
	log.Printf("Load balancer started on port %s", lb.Port)
	log.Printf("Backends: %v", cfg.Servers)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func runConfigUpdater(ctx context.Context, lb *balancer.RoundRobin, cfg *balancer.ConfigData) {
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
				log.Printf("Config reloaded successfully. Backends: %v", cfg.Servers)
			}

		case <-ctx.Done():
			log.Println("Stopping config updater")
			return
		}
	}
}

func handleShutdown(server *http.Server, cancel context.CancelFunc) {
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

// надо сделать хелфчек перед тем, как вызывать некст сервак
// добавить логи
// сделать по тикеру обновление списка серваков
// залогировать как то удаление старых серваков и добавление новых
