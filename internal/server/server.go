/*
Copyright (c) 2024 Ansible Project

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/work-obs/ansible-go/internal/auth"
	"github.com/work-obs/ansible-go/pkg/api"
	"github.com/work-obs/ansible-go/pkg/config"
)

// Server represents the Ansible Go HTTPS server
type Server struct {
	httpServer *http.Server
	jwtManager *auth.JWTManager
	config     *config.Config
	router     *gin.Engine
}

// Config holds server configuration
type Config struct {
	Host         string
	Port         int
	TLSCertFile  string
	TLSKeyFile   string
	JWTIssuer    string
	JWTAudience  []string
	JWTTokenTTL  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// NewServer creates a new Ansible Go server instance
func NewServer(cfg *Config, ansibleConfig *config.Config) (*Server, error) {
	// Create JWT manager
	jwtManager, err := auth.NewJWTManager(cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	// Set Gin to release mode in production
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware())

	// Create server instance
	server := &Server{
		jwtManager: jwtManager,
		config:     ansibleConfig,
		router:     router,
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
	}

	return server, nil
}

// Start starts the HTTPS server
func (s *Server) Start(certFile, keyFile string) error {
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("TLS certificate and key files are required")
	}

	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check endpoint (no auth required)
	s.router.GET("/health", s.healthCheck)

	// API v1 routes with JWT authentication
	v1 := s.router.Group("/api/v1")
	v1.Use(s.authMiddleware())
	{
		// Playbook operations
		v1.POST("/playbook/execute", s.executePlaybook)
		v1.GET("/playbook/status/:execution_id", s.getPlaybookStatus)

		// Module operations
		v1.POST("/module/execute", s.executeModule)

		// Inventory operations
		v1.GET("/inventory", s.getInventory)
		v1.GET("/inventory/hosts", s.listHosts)
		v1.GET("/inventory/groups", s.listGroups)

		// Configuration operations
		v1.GET("/config", s.getConfiguration)

		// Plugin operations
		v1.GET("/plugins", s.listPlugins)
		v1.GET("/plugins/:plugin_type/:plugin_name", s.getPluginInfo)

		// Router operations
		v1.GET("/router/config", s.getRouterConfig)
		v1.PUT("/router/config", s.updateRouterConfig)
	}
}

// authMiddleware handles JWT authentication
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, api.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Extract Bearer token
		const bearerPrefix = "Bearer "
		if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
			c.JSON(http.StatusUnauthorized, api.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		token := authHeader[len(bearerPrefix):]
		claims, err := s.jwtManager.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, api.ErrorResponse{
				Code:    http.StatusUnauthorized,
				Message: "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Store claims in context
		c.Set("claims", claims)
		c.Next()
	}
}

// loggingMiddleware provides request logging
func loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
				param.ClientIP,
				param.TimeStamp.Format(time.RFC3339),
				param.Method,
				param.Path,
				param.Request.Proto,
				param.StatusCode,
				param.Latency,
				param.Request.UserAgent(),
				param.ErrorMessage,
			)
		},
		Output:    gin.DefaultWriter,
		SkipPaths: []string{"/health"},
	})
}

// healthCheck handles the health check endpoint
func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"version":   "2.19.0-go",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// executePlaybook handles playbook execution requests
func (s *Server) executePlaybook(c *gin.Context) {
	var req api.PlaybookExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request format",
			Details: map[string]interface{}{"error": err.Error()},
		})
		return
	}

	// TODO: Implement playbook execution
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Playbook execution not yet implemented",
	})
}

// getPlaybookStatus handles playbook status requests
func (s *Server) getPlaybookStatus(c *gin.Context) {
	executionID := c.Param("execution_id")
	if executionID == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Execution ID is required",
		})
		return
	}

	// TODO: Implement playbook status retrieval
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Playbook status retrieval not yet implemented",
	})
}

// executeModule handles module execution requests
func (s *Server) executeModule(c *gin.Context) {
	var req api.ModuleExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request format",
			Details: map[string]interface{}{"error": err.Error()},
		})
		return
	}

	// TODO: Implement module execution
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Module execution not yet implemented",
	})
}

// getInventory handles inventory retrieval requests
func (s *Server) getInventory(c *gin.Context) {
	// TODO: Implement inventory retrieval
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Inventory retrieval not yet implemented",
	})
}

// listHosts handles host listing requests
func (s *Server) listHosts(c *gin.Context) {
	// TODO: Implement host listing
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Host listing not yet implemented",
	})
}

// listGroups handles group listing requests
func (s *Server) listGroups(c *gin.Context) {
	// TODO: Implement group listing
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Group listing not yet implemented",
	})
}

// getConfiguration handles configuration retrieval requests
func (s *Server) getConfiguration(c *gin.Context) {
	// TODO: Implement configuration retrieval
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Configuration retrieval not yet implemented",
	})
}

// listPlugins handles plugin listing requests
func (s *Server) listPlugins(c *gin.Context) {
	// TODO: Implement plugin listing
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Plugin listing not yet implemented",
	})
}

// getPluginInfo handles plugin information requests
func (s *Server) getPluginInfo(c *gin.Context) {
	pluginType := c.Param("plugin_type")
	pluginName := c.Param("plugin_name")

	if pluginType == "" || pluginName == "" {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Plugin type and name are required",
		})
		return
	}

	// TODO: Implement plugin info retrieval
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Plugin info retrieval not yet implemented",
	})
}

// getRouterConfig handles router configuration retrieval requests
func (s *Server) getRouterConfig(c *gin.Context) {
	// TODO: Implement router config retrieval
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Router config retrieval not yet implemented",
	})
}

// updateRouterConfig handles router configuration update requests
func (s *Server) updateRouterConfig(c *gin.Context) {
	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid configuration format",
			Details: map[string]interface{}{"error": err.Error()},
		})
		return
	}

	// TODO: Implement router config update
	c.JSON(http.StatusNotImplemented, api.ErrorResponse{
		Code:    http.StatusNotImplemented,
		Message: "Router config update not yet implemented",
	})
}