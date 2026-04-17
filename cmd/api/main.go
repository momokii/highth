// Package main is the entry point for the IoT Sensor Query API.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kelanach/higth/internal/cache"
	"github.com/kelanach/higth/internal/config"
	"github.com/kelanach/higth/internal/handler"
	"github.com/kelanach/higth/internal/repository"
	higthmiddleware "github.com/kelanach/higth/internal/middleware"
	"github.com/kelanach/higth/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize repository
	repo, err := repository.New(repository.Config{
		DatabaseURL:  cfg.DatabaseURL,
		MaxConnLifetime:   cfg.DBMaxConnLifetime,
		MaxConnIdleTime:   cfg.DBMaxConnIdleTime,
		HealthCheckPeriod: cfg.DBHealthCheckPeriod,
		MaxOpenConns: int32(cfg.DBMaxConnections),
		MinOpenConns: int32(cfg.DBMinConnections),
	})
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize cache
	redisCache, err := cache.New(cache.Config{
		URL:             cfg.RedisURL,
		Enabled:         cfg.RedisEnabled,
		TTL:             cfg.RedisTTL,
		PoolSize:        cfg.RedisPoolSize,
		MinIdleConns:    cfg.RedisMinIdleConns,
		MaxIdleConns:    cfg.RedisMaxIdleConns,
		ConnMaxIdleTime: cfg.RedisConnMaxIdleTime,
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize cache: %v", err)
		// Continue without cache
		redisCache = &cache.RedisCache{}
	}
	defer func() { _ = redisCache.Close() }()

	// Initialize service
	sensorService := service.New(repo, redisCache, service.Config{
		CacheEnabled: cfg.CacheEnabled,
	})

	// Initialize handlers
	sensorHandler := handler.NewSensorHandler(sensorService)
	healthHandler := handler.NewHealthHandler(sensorService)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(cfg.RequestTimeout))
	r.Use(higthmiddleware.GzipMiddleware)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))
	r.Use(higthmiddleware.MetricsMiddleware)

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/sensor-readings", sensorHandler.GetSensorReadings)
		r.Get("/stats", sensorHandler.GetStats)
	})

	// Health check routes
	r.Get("/health", healthHandler.GetHealth)
	r.Get("/health/ready", healthHandler.GetReadiness)
	r.Get("/health/live", healthHandler.GetLiveness)

	// Metrics endpoint for Prometheus
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		higthmiddleware.MetricsHandler().ServeHTTP(w, r)
	})

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
