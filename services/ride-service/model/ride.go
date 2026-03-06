package model

import (
	"database/sql"
	"time"
)

type Ride struct {
	ID                 string          `json:"id"`
	RiderID            string          `json:"rider_id"`
	DriverID           sql.NullString  `json:"driver_id"`
	Status             string          `json:"status"`
	PickupLatitude     float64         `json:"pickup_latitude"`
	PickupLongitude    float64         `json:"pickup_longitude"`
	PickupAddress      string          `json:"pickup_address"`
	DropoffLatitude    float64         `json:"dropoff_latitude"`
	DropoffLongitude   float64         `json:"dropoff_longitude"`
	DropoffAddress     string          `json:"dropoff_address"`
	VehicleType        string          `json:"vehicle_type"`
	EstimatedFare      float64         `json:"estimated_fare"`
	ActualFare         float64         `json:"actual_fare"`
	DistanceKm         float64         `json:"distance_km"`
	DurationMinutes    float64         `json:"duration_minutes"`
	SurgeMultiplier    float64         `json:"surge_multiplier"`
	PaymentMethod      string          `json:"payment_method"`
	RiderRating        sql.NullFloat64 `json:"rider_rating"`
	DriverRating       sql.NullFloat64 `json:"driver_rating"`
	CancellationReason string          `json:"cancellation_reason"`
	CancelledBy        sql.NullString  `json:"cancelled_by"`
	CreatedAt          time.Time       `json:"created_at"`
	AcceptedAt         sql.NullTime    `json:"accepted_at"`
	StartedAt          sql.NullTime    `json:"started_at"`
	CompletedAt        sql.NullTime    `json:"completed_at"`
	CancelledAt        sql.NullTime    `json:"cancelled_at"`
}
