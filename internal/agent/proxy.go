// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"fmt"
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
}

// NewEnvProxy creates a new reverse proxy listening on the given address
func NewEnvProxy(listenAddr string) (*EnvProxy, error) {
	p := &EnvProxy{
		routes: make(map[string]*url.URL),
	}

	p.server = &http.Server{
		Addr:    listenAddr,
		Handler: http.HandlerFunc(p.handleRequest),
	}

	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't fail - proxy is optional
		}
	}()

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
