// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// EnvProxy is a reverse proxy that routes HTTP traffic by Host header
type EnvProxy struct {
	routes map[string]*url.URL
	mu     sync.RWMutex
	server *http.Server
	ln     net.Listener
}

// NewEnvProxy creates a new reverse proxy listening on the given address
func NewEnvProxy(listenAddr string) (*EnvProxy, error) {
	p := &EnvProxy{
		routes: make(map[string]*url.URL),
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	p.ln = ln

	p.server = &http.Server{Handler: http.HandlerFunc(p.handleRequest)}

	go func() {
		if err := p.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("proxy serve error: %v", err)
		}
	}()

	log.Printf("proxy listening on %s", listenAddr)
	return p, nil
}

// handleRequest routes requests based on Host header
func (p *EnvProxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}

	// Strip port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	p.mu.RLock()
	target, ok := p.routes[host]
	p.mu.RUnlock()

	if !ok {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "502 Bad Gateway: no route for %s\n", host)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "502 Bad Gateway: %v\n", err)
	}

	proxy.ServeHTTP(w, r)
}

// AddRoute registers a new route for the given hostname
func (p *EnvProxy) AddRoute(hostname, target string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	p.mu.Lock()
	p.routes[hostname] = targetURL
	p.mu.Unlock()

	log.Printf("proxy route: %s -> %s", hostname, target)

	return nil
}

// RemoveRoutesForEnv removes all routes for a given environment
func (p *EnvProxy) RemoveRoutesForEnv(envName string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	suffix := "." + envName
	for host := range p.routes {
		if strings.Contains(host, suffix) {
			delete(p.routes, host)
			log.Printf("proxy route removed: %s", host)
		}
	}
}

// Close shuts down the proxy server
func (p *EnvProxy) Close() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}
