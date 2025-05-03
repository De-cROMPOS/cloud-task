package balancer

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"sync"
	"time"
)

// Interface to generate balancer structs
type Balancer interface {
	UpdateServers(*ConfigData) error
	Generate() (*backendServer, error)
}

// Roundrobin structure
type RoundRobin struct {
	mu      sync.RWMutex
	Port    string
	servers []*backendServer
	pointer int
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{
		mu:      sync.RWMutex{},
		servers: make([]*backendServer, 0),
		pointer: 0,
	}
}

// Generating servers by RoundRobin algorithm
func (r *RoundRobin) Generate() (*backendServer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	serversLen := len(r.servers)

	r.pointer++
	if r.pointer >= serversLen {
		r.pointer = 0
	}

	for range serversLen {

		backServ := r.servers[r.pointer]

		//checking if the server is available
		if (backServ.IsAlive) && !(r.isServerAvailable(backServ.Addr)) {
			backServ.markUnavailable(1 * time.Minute)
			log.Printf("server %v unavailable at the moment, turning it off for a minute", backServ.Addr.Host)
		}

		//returning server if it works, moving to the next server if not
		if backServ.IsAlive {
			return backServ, nil
		} else {
			r.pointer++
			if r.pointer >= serversLen {
				r.pointer = 0
			}
		}
	}

	return nil, fmt.Errorf("no server available")
}

// Pinging server to check if its available
func (r *RoundRobin) isServerAvailable(url *url.URL) bool {
	conn, err := net.DialTimeout("tcp", url.Host, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Updating server list from config
func (r *RoundRobin) UpdateServers(cd *ConfigData) error {

	if (len(cd.Servers) == 0) {
		return fmt.Errorf("no servers in config file, please add")
	}

	// Enum for server status
	type updateStatus int
	const (
		serverNew updateStatus = iota
		serverExisted
	)

	newServerArr := make([]*backendServer, 0)
	serversMap := make(map[string]updateStatus)

	for _, el := range cd.Servers {
		serversMap[el] = serverNew
	}

	r.mu.Lock()

	r.Port = cd.Port

	// removing unused servers
	for _, serversEl := range r.servers {
		host := serversEl.Addr.Host
		if _, exists := serversMap[host]; exists {
			newServerArr = append(newServerArr, serversEl)
			serversMap[host] = serverExisted
			// log.Printf("server %v kept in balancer", host)
		} else {
			log.Printf("server %v removed from balancer by cfg file", serversEl.Addr.Host)
		}
	}
	r.mu.Unlock()

	// Adding new servers
	for serverHost, status := range serversMap {
		if status == serverNew {
			parsedURL, err := url.Parse("http://" + serverHost)
			if err != nil {
				log.Printf("error parsing server URL %v: %v", serverHost, err)
				continue
			}

			newServerArr = append(newServerArr, &backendServer{
				Addr:    parsedURL,
				IsAlive: true,
			})
			log.Printf("server %v added to balancer", serverHost)
		}
	}

	r.mu.Lock()
	r.servers = newServerArr
	r.mu.Unlock()

	return nil
}
