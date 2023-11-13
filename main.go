package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/time/rate"
)

// Config represents the rate limit configuration.
type Config struct {
	Limit  float64       // Maximum number of events allowed per second
	Burst  int           // Maximum number of events that can be sent in a single burst
	Period time.Duration // Time window for rate limiting
}

// RateStats represents the statistics for rate limiting.
type RateStats struct {
	StartTime time.Time
	Success   int
	Exceeded  int
	mu        sync.Mutex
}

// NewLimiter creates a new rate limiter based on the provided configuration.
func NewLimiter(config Config) *rate.Limiter {
	return rate.NewLimiter(rate.Limit(config.Limit), config.Burst)
}

// RateLimitMiddleware is a Fiber middleware that performs rate limiting and updates statistics.
func RateLimitMiddleware(config Config, stats *RateStats) func(*fiber.Ctx) error {
	limiter := NewLimiter(config)

	return func(c *fiber.Ctx) error {
		stats.mu.Lock()
		defer stats.mu.Unlock()	

		if CheckRateLimit(limiter, 1) {
			stats.Success++
			return c.SendStatus(fiber.StatusOK)
		}

		stats.Exceeded++
		return c.SendStatus(fiber.StatusTooManyRequests)
	}
}

// CheckRateLimit checks if an event is allowed based on the rate limiter.
func CheckRateLimit(limiter *rate.Limiter, eventCount int) bool {
	return limiter.AllowN(time.Now(), eventCount)
}

func main() {
	app := fiber.New()

	// Example rate limit configuration
	config := Config{
		Limit:  2,            // 2 events per second
		Burst:  4,            // Allowing bursts of up to 4 events
		Period: time.Second,   // 1 second time window
	}

	// Initialize rate limiting statistics
	stats := &RateStats{
		StartTime: time.Now(),
	}

	// Apply rate limiting middleware
	app.Use(RateLimitMiddleware(config, stats))

	// Define your route
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, Fiber!")
	})

	// Start the server
	go func() {
		err := app.Listen(":3000")
		if err != nil {
			fmt.Println("Error starting server:", err)
			os.Exit(1)
		}
	}()

	// Capture SIGINT signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT)

	// Wait for the signal
	<-sigChan

	// Print statistics before exiting
	stats.mu.Lock()
	defer stats.mu.Unlock()
	elapsed := int(time.Since(stats.StartTime).Seconds())
	fmt.Printf("\nElapsed time: %d seconds | 200 OK: %d | 429 Too Many Requests: %d\n", elapsed, stats.Success, stats.Exceeded)
	os.Exit(0)
}
