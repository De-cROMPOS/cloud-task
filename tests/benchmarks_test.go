package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"main/balancer"
	"main/handlers"
	redisdb "main/redisDB"
)


// To use this benchmarks you should start Redis server and some servers in imitator
// ./run.sh redis - to start Redis in docker
// ./run.sh imitator - to start imitator
// ./run.sh bench to start benchmarking or (go test -bench=BenchmarkCreateAndDeleteClient ./tests -cpuprofile="./tests/benchmarks-output/CPU.out" -memprofile="./tests/benchmarks-output/MEM.out")
// After generating out files you can look at the statistic:
// (go tool pprof MEM.out) - to open tool 
// (list funcName) to see benchmark info at chosen function 

func BenchmarkBalancerWithClientSetup(b *testing.B) {
	// Initializing
	redisClient := redisdb.NewClient("localhost:6379")
	lb := balancer.NewRoundRobin()
	cfg := balancer.NewConfigData()
	cfg.Servers = []string{
		"127.0.0.1:8081",
		"127.0.0.1:8082",
		"127.0.0.1:8083",
		"127.0.0.1:8085",
		"127.0.0.1:8084",
	}
	cfg.Port = "8080"

	// Loading data to our balancer
	if err := lb.UpdateServers(cfg); err != nil {
		b.Fatalf("Failed to update servers: %v", err)
	}

	// Making handlers
	dbHandler := handlers.NewDBHandler(redisClient)
	balancerHandler := handlers.NewBalancerHandler(redisClient, lb)

	// Initializing client data
	clientData := map[string]interface{}{
		"api_key":      "test123",
		"capacity":     10000,
		"rate_per_sec": 1000,
	}
	jsonData, _ := json.Marshal(clientData)

	// Posting client data
	reqCreate, _ := http.NewRequest("POST", "http://localhost:8080/clients", bytes.NewBuffer(jsonData))
	reqCreate.Header.Set("Content-Type", "application/json")
	rrCreate := httptest.NewRecorder()
	dbHandler.ServeHTTP(rrCreate, reqCreate)

	// Requests for testing
	reqBalance, _ := http.NewRequest("GET", "http://localhost:8080/", nil)
	reqBalance.Header.Set("api-key", "test123")
	rrBalance := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		balancerHandler.ServeHTTP(rrBalance, reqBalance)
	}
}

func BenchmarkCreateAndDeleteClient(b *testing.B) {
	// Initializing 
    redisClient := redisdb.NewClient("localhost:6379")
    handler := handlers.NewDBHandler(redisClient)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        key := fmt.Sprintf("test%d", i)
        
        // Benchmarking creating new clients
        createData := map[string]interface{}{
            "api_key":      key,
            "capacity":     100,
            "rate_per_sec": 10,
        }
        jsonData, _ := json.Marshal(createData)
        
        createReq := httptest.NewRequest("POST", "/clients", bytes.NewBuffer(jsonData))
        createReq.Header.Set("Content-Type", "application/json")
        createRr := httptest.NewRecorder()
        handler.ServeHTTP(createRr, createReq)

        // Benchmarking deleting clients
        deleteReq := httptest.NewRequest("DELETE", "/clients?api_key="+key, nil)
        deleteRr := httptest.NewRecorder()
        handler.ServeHTTP(deleteRr, deleteReq)
    }
}