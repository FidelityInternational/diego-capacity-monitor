package webServer

import (
	"github.com/FidelityInternational/diego-capacity-monitor/metrics"
	"github.com/gorilla/mux"
	"time"
)

// Server struct
type Server struct {
	Controller *Controller
}

// CreateServer - creates a server
func CreateServer(metrics metrics.Metrics, cellMemory *float64, watermark *string) *Server {
	startTime := time.Now()
	controller := CreateController(metrics, cellMemory, watermark, startTime)

	return &Server{
		Controller: controller,
	}
}

// Start - starts the web server
func (s *Server) Start() *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/", s.Controller.Index).Methods("GET")

	return router
}
