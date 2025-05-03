package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"

	"main/balancer"
	redisdb "main/redisDB"
)

// Struct to return errors like { "code": 429, "message": "Rate limit exceeded" }
type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func errorResponser(w http.ResponseWriter, message string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(errorResponse{
        Code:    code,
        Message: message,
    })
	log.Printf("An error happened while processing handler: %v. Exit code: %v", message, code)
}

// Redis handler
type DBHandler struct {
	redisClient *redisdb.Client
}

func NewDBHandler(redisClient *redisdb.Client) *DBHandler {
	return &DBHandler{redisClient: redisClient}
}

func (h *DBHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var client struct {
			APIKey   string `json:"api_key"`
			Capacity int    `json:"capacity"`
			Rate     int    `json:"rate_per_sec"`
		}

		// Setting up default values
		client.Capacity = 100
        client.Rate = 10

		if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
			errorResponser(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validation
		if client.APIKey == "" {
            errorResponser(w, "API key is required", http.StatusBadRequest)
            return
        }
		
		// Adding client to redis DB
		if err := h.redisClient.AddClient(r.Context(), client.APIKey, client.Capacity, client.Rate); err != nil {
			errorResponser(w, "Failed to add client", http.StatusInternalServerError)
			return
		}

		log.Printf("Added client %v with cap = %v and rate per sec = %v", client.APIKey, client.Capacity, client.Rate)

		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		apiKey := r.URL.Query().Get("api_key")
		if apiKey == "" {
			errorResponser(w, "API key required", http.StatusBadRequest)
			return
		}

		// Deleting client from DB
		if err := h.redisClient.DeleteClient(r.Context(), apiKey); err != nil {
			errorResponser(w, "Failed to delete client", http.StatusInternalServerError)
			return
		}

		log.Printf("Deleted client %v", apiKey)

		w.WriteHeader(http.StatusOK)

	default:
		errorResponser(w, "Method not allowed", http.StatusBadRequest)
	}
}

// Balancer handler
type balancerHandler struct {
	redisClient *redisdb.Client
	balancer    *balancer.RoundRobin
}

func NewBalancerHandler(redisClient *redisdb.Client, server *balancer.RoundRobin) *balancerHandler {
	return &balancerHandler{redisClient: redisClient, balancer: server}
}

func (h *balancerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("api-key")
	if apiKey == "" {
		errorResponser(w, "API key required", http.StatusUnauthorized)
		return
	}

	// Checking if our api  key exists in DB and if it has enough tokens
	allowed, err := h.redisClient.TakeToken(r.Context(), apiKey)
	if err != nil {
		errorResponser(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !allowed {
		errorResponser(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// Reversing the request if everything is ok
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			server, err := h.balancer.Generate()
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
	proxy.ServeHTTP(w, r)
}
