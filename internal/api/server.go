// Package api exposes the fleet REST API.
package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"transjakarta-fleet/internal/database"
)

// Server serves the REST API on top of the storage layer.
type Server struct {
	db *database.DB
}

func NewServer(db *database.DB) *Server {
	return &Server{db: db}
}

func (s *Server) Router() *gin.Engine {
	router := gin.Default()

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	vehicles := router.Group("/vehicles")
	{
		vehicles.GET("/:vehicle_id/location", s.latestLocation)
		vehicles.GET("/:vehicle_id/history", s.history)
	}
	return router
}

// latestLocation serves GET /vehicles/{vehicle_id}/location.
func (s *Server) latestLocation(c *gin.Context) {
	vehicleID := c.Param("vehicle_id")

	loc, err := s.db.GetLatestLocation(c.Request.Context(), vehicleID)
	switch {
	case errors.Is(err, database.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("no location found for vehicle %q", vehicleID),
		})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, loc)
	}
}

// history serves GET /vehicles/{vehicle_id}/history?start=...&end=...&limit=...
// start and end are unix seconds (inclusive range). limit is optional,
// defaults to 500 and is capped at 1000.
func (s *Server) history(c *gin.Context) {
	vehicleID := c.Param("vehicle_id")

	start, ok := parseUnixParam(c, "start")
	if !ok {
		return
	}
	end, ok := parseUnixParam(c, "end")
	if !ok {
		return
	}
	if start >= end {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'start' must be lower than 'end'"})
		return
	}

	limit := 500
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "'limit' must be a positive integer"})
			return
		}
		if n > 1000 {
			n = 1000
		}
		limit = n
	}

	locations, err := s.db.GetHistory(c.Request.Context(), vehicleID, start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"vehicle_id": vehicleID,
		"start":      start,
		"end":        end,
		"count":      len(locations),
		"locations":  locations,
	})
}

// parseUnixParam reads a required unix-seconds query parameter and writes a
// 400 response itself when it is missing or malformed.
func parseUnixParam(c *gin.Context, name string) (int64, bool) {
	v := c.Query(name)
	if v == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("query parameter '%s' is required (unix seconds)", name),
		})
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("query parameter '%s' must be unix seconds", name),
		})
		return 0, false
	}
	return n, true
}
