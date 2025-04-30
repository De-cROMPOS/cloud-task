package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	basePort    = 8081
	serverTimeout = 1 * time.Second
)

// Starting http server on user port
func startServer(serverInd int, wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()

	mux := http.NewServeMux()

	serverPort := basePort + serverInd

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		resp := "Hello, I'm server №" + strconv.Itoa(serverInd+1)
		w.Write([]byte(resp))
	})

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(serverPort),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()

		fmt.Printf("Server %v says bye-bye\n", serverInd+1)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Server %v got an error while closing %v\n", serverInd+1, err)
		}
	}()

	fmt.Printf("Server %v started on port %v\n", serverInd+1, serverPort)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("Server %v failed while starting: %v\n", serverInd+1, err)
	}
}

func main() {
	var ammount int

	fmt.Print("Type ammount of servers: ")
	if _, err := fmt.Scan(&ammount); err != nil {
		fmt.Println("error while getting ammount of users:", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	for i := range ammount {
		wg.Add(1)
		go startServer(i, &wg, ctx)
	}

	// Timeout to let the servers start
	time.Sleep(serverTimeout)
	fmt.Println("Type CTRL+C to stop all the servers ")

	wg.Wait()
	fmt.Println("All the servers are shut down")
}
