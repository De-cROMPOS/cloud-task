package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"main/balancer"
	"main/handlers"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	redisdb "main/redisDB"
)

// Testing http server
type testServer struct {
	URL   string
	Close func()
}

func startTestServer(port int, response string) testServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}

	go server.ListenAndServe()

	return testServer{
		URL:   "localhost:" + strconv.Itoa(port),
		Close: func() { server.Close() },
	}
}

func TestRoundRobin(t *testing.T) {
	// Initializing test servers
	servers := []struct {
		name    string
		port    int
		message string
	}{
		{"Server1", 8081, "Server 1"},
		{"Server2", 8082, "Server 2"},
		{"Server3", 8083, "Server 3"},
	}

	var testServers []testServer
	for _, s := range servers {
		srv := startTestServer(s.port, s.message)
		testServers = append(testServers, srv)
	}

	var serverURLs []string
	for _, s := range testServers {
		serverURLs = append(serverURLs, s.URL)
	}

	// Initializing balancer and cfg
	lb := balancer.NewRoundRobin()
	cfg := balancer.NewConfigData()
	cfg.Servers = serverURLs

	// Testing updateServer func
	t.Run("UpdateServers", func(t *testing.T) {
		if err := lb.UpdateServers(cfg); err != nil {
			t.Fatalf("UpdateServers failed: %v", err)
		}
	})

	// Checking if the order of our Balancer's generator works good
	t.Run("RoundRobinOrder", func(t *testing.T) {
		// Scrolling until first server
		server, err := lb.Generate()
		if err != nil {
			t.Fatalf("Generating server failed: %v", err)
		}
		for range 3 { // There are 3 servers so there will be maximum 3 generate operations
			if server.Addr.Host != testServers[0].URL {
				server, err = lb.Generate()
				if err != nil {
					t.Fatalf("Generating server failed: %v", err)
				}
			}
		}

		// Checking order of RoundRobin servers generator
		expectedOrder := []string{
			testServers[1].URL, testServers[2].URL, testServers[0].URL,
			testServers[1].URL, testServers[2].URL, testServers[0].URL,
			testServers[1].URL, testServers[2].URL, testServers[0].URL,
		}

		for i, expected := range expectedOrder {
			server, err := lb.Generate()
			if err != nil {
				t.Fatalf("Generating server failed: %v", err)
			}

			if server.Addr.Host != expected {
				t.Errorf("Iteration %d: expected %s, got %s", i, expected, server.Addr.Host)
			}
		}
	})

	// Checking server updater in RoundRobin
	t.Run("DynamicServerUpdate", func(t *testing.T) {

		// Adding new server
		newServer := startTestServer(8084, "Server 4")
		defer newServer.Close()

		// Making new config server list and updating balancer's server info
		cfg.Servers = []string{newServer.URL, testServers[2].URL}
		if err := lb.UpdateServers(cfg); err != nil {
			t.Fatalf("UpdateServers failed: %v", err)
		}

		// Checking if the balancer uses new servers (Server 3 and Server 4)
		server, err := lb.Generate()
		if err != nil {
			t.Fatalf("Generating server failed: %v", err)
		}
		if server.Addr.Host != newServer.URL && server.Addr.Host != testServers[2].URL {
			t.Errorf("Expected server from updated list, got %s", server.Addr.Host)
		}
	})

	// Trying to update empty set of servers
	t.Run("EmptyServerList", func(t *testing.T) {
		cfg.Servers = []string{}
		if err := lb.UpdateServers(cfg); err == nil {
			t.Error("Expected error for empty server list")
		}
	})

	// All the servers are unavailable at the moment (closed)
	t.Run("AllServersUnavailable", func(t *testing.T) {
		for _, s := range testServers {
			s.Close()
		}

		if _, err := lb.Generate(); err == nil {
			t.Error("Expected error no servers available")
		}
	})

	// Testing getting data from config file
	t.Run("GettingDataFromCFGFile", func(t *testing.T) {
		err := cfg.GetCfgData()
		if err != nil {
			t.Fatalf("Getting data from cfg file failed: %v", err)
		}
	})
}

func TestBalancerWithRedis(t *testing.T) {
	// Client initializing
	redisClient := redisdb.NewClient("localhost:6379")

	if err := redisClient.Ping(context.Background()); err != nil {
		t.Fatalf("Redis connection failed: %v", err)
	}

	// Initializing balancer
	lb := balancer.NewRoundRobin()
	cfg := balancer.NewConfigData()
	cfg.Servers = []string{"localhost:8081", "localhost:8082"}
	cfg.Port = "8080"

	if err := lb.UpdateServers(cfg); err != nil {
		t.Fatalf("Failed to update servers: %v", err)
	}

	// Making test servers
	server1 := startTestServer(8081, "test server 1")
	defer server1.Close()

	server2 := startTestServer(8082, "test server 2")
	defer server2.Close()

	// Making multiplexer
	mux := http.NewServeMux()
	mux.Handle("/clients", handlers.NewDBHandler(redisClient))
	mux.Handle("/", handlers.NewBalancerHandler(redisClient, lb))

	// Starting test server
	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// Testing post method in db handler
	t.Run("ClientPostOperations", func(t *testing.T) {
		testCases := []struct {
			name       string
			data       map[string]interface{}
			expectCode int
		}{
			{
				name: "Valid client creation",
				data: map[string]interface{}{
					"api_key":      "test123",
					"capacity":     100,
					"rate_per_sec": 10,
				},
				expectCode: http.StatusCreated,
			},
			{
				name: "Invalid data structure",
				data: map[string]interface{}{
					"some":      "test123",
					"random":    100,
					"incorrect": 10,
					"data":      123123123,
				},
				expectCode: http.StatusBadRequest,
			},
			{
				name: "Missing required fields",
				data: map[string]interface{}{
					"api_key": "test456",
				},
				expectCode: http.StatusCreated,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				jsonData, err := json.Marshal(tc.data)
				if err != nil {
					t.Fatalf("JSON marshaling failed: %v", err)
				}

				resp, err := http.Post(
					testServer.URL+"/clients",
					"application/json",
					bytes.NewBuffer(jsonData),
				)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != tc.expectCode {
					t.Errorf("%s: expected status %d, got %d", tc.name, tc.expectCode, resp.StatusCode)
				}
			})
		}
	})

	// Testing delete method in db handler
	t.Run("ClientDeleteOperations", func(t *testing.T) {

		// Making test client
		createValidClient(t, testServer.URL, "test123", 100 , 10)

		testCases := []struct {
			name       string
			apiKey     string
			expectCode int
		}{
			{
				name:       "Valid deletion",
				apiKey:     "test123",
				expectCode: http.StatusOK,
			},
			{
				name:       "Missing API key",
				apiKey:     "",
				expectCode: http.StatusBadRequest,
			},
			{
				name:       "Non existent client",
				apiKey:     "nonexistent_key",
				expectCode: http.StatusOK,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				url := testServer.URL + "/clients"
				if tc.apiKey != "" {
					url += "?api_key=" + tc.apiKey
				}

				req, err := http.NewRequest("DELETE", url, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != tc.expectCode {
					t.Errorf("%s: expected status %d, got %d",
						tc.name, tc.expectCode, resp.StatusCode)
				}

			})
		}
	})

	// Testing balancer handler
	t.Run("GettingServerOperations", func(t *testing.T) {

		// Adding test client
		createValidClient(t, testServer.URL, "test123", 100, 10)
		createValidClient(t, testServer.URL, "test456", 0, 0) // For to many requests status
	
		testCases := []struct {
			name        string
			apiKey      string
			expectCode  int
		}{
			{
				name:        "Valid API key",
				apiKey:      "test123",
				expectCode:  http.StatusOK,
			},
			{
				name:        "Nonexistent API key",
				apiKey:      "nonexistentKey",
				expectCode:  http.StatusTooManyRequests,
			},
			{
				name:        "Zero capacity API key",
				apiKey:      "test456",
				expectCode:  http.StatusTooManyRequests,
			},
		}
	
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req, err := http.NewRequest("GET", testServer.URL, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}
	
				if tc.apiKey != "" {
					req.Header.Set("api-key", tc.apiKey)
				}
	
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()
	
				if resp.StatusCode != tc.expectCode {
					t.Errorf("%s: expected status %d, got %d", tc.name, tc.expectCode, resp.StatusCode)
				}
			})
		}
	})
}


func createValidClient(t *testing.T, baseURL, apiKey string, capacity, rate int) {
	clientData := map[string]interface{}{
		"api_key":      apiKey,
		"capacity":     capacity,
		"rate_per_sec": rate,
	}
	jsonData, _ := json.Marshal(clientData)

	resp, err := http.Post(
		baseURL+"/clients",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to create test client, status: %d", resp.StatusCode)
	}
}