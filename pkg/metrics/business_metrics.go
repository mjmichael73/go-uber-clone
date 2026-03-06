package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Example business metrics
	RidesCreated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uber_rides_created_total",
		Help: "The total number of created rides",
	})

	RidesCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "uber_rides_completed_total",
		Help: "The total number of completed rides",
	})

	DriverStatusUpdates = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "uber_driver_status_updates_total",
		Help: "The total number of driver status updates",
	}, []string{"status"})
)
