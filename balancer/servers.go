package balancer

import (
	"log"
	"net/url"
	"sync"
	"time"
)

type backendServer struct {
	Addr    *url.URL
	IsAlive bool
	recoveryTimer *time.Timer
	mu sync.Mutex
}

//setting freeze on unavailable server
func (s *backendServer) markUnavailable(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.IsAlive = false

	if s.recoveryTimer != nil {
		s.recoveryTimer.Stop()
	}

	//setting freeze timer
	s.recoveryTimer = time.AfterFunc(duration, func() {
		s.mu.Lock()
		s.IsAlive = true
		s.recoveryTimer = nil
		log.Printf("server %v cooldown ended, now it's available again", s.Addr)
		s.mu.Unlock()
	})
}
