package server

import (
	"math/rand"
	"monitoring/db"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	router *gin.Engine
	db     *db.Database
}

// NewServer creates a new server instance
func NewServer(database *db.Database) *Server {
	router := gin.Default()
	server := &Server{
		router: router,
		db:     database,
	}
	server.setupRoutes()
	return server
}

// setupRoutes configures all the routes
func (s *Server) setupRoutes() {
	s.router.GET("/report", s.handleReport)
	s.router.GET("/metrics", s.handleGetMetrics)
	s.router.GET("/metrics/:ip", s.handleGetMetricByIp)
}

// handleReport generates a random metric and stores it
func (s *Server) handleReport(c *gin.Context) {
	value := rand.Intn(100)
	ip := "10.0.0.1"

	err := s.db.CreateMetric(ip, value)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to store metric"})
		return
	}

	c.String(200, strconv.Itoa(value))
}

// handleGetMetrics returns all metrics
func (s *Server) handleGetMetrics(c *gin.Context) {
	metrics, err := s.db.GetMetrics()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to retrieve metrics"})
		return
	}

	c.JSON(200, metrics)
}

// handleGetMetricByIp returns a specific metric by ip
func (s *Server) handleGetMetricByIp(c *gin.Context) {
	ip := c.Param("ip")
	metric, err := s.db.GetMetricByIP(ip)
	if err != nil {
		c.JSON(404, gin.H{"error": "Metric not found"})
		return
	}

	c.JSON(200, metric)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	serverPort := os.Getenv("PORT")
	if serverPort == "" {
		serverPort = "8080"
	}
	return s.router.Run(":" + serverPort)
}
