package model

import "time"

type Driver struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	LicenseNumber    string    `json:"license_number"`
	VehicleMake      string    `json:"vehicle_make"`
	VehicleModel     string    `json:"vehicle_model"`
	VehicleYear      string    `json:"vehicle_year"`
	VehicleColor     string    `json:"vehicle_color"`
	PlateNumber      string    `json:"plate_number"`
	VehicleType      string    `json:"vehicle_type"`
	Status           string    `json:"status"`
	Rating           float32   `json:"rating"`
	TotalRides       int       `json:"total_rides"`
	TotalRatings     int       `json:"total_ratings"`
	CurrentLatitude  float64   `json:"current_latitude"`
	CurrentLongitude float64   `json:"current_longitude"`
	IsVerified       bool      `json:"is_verified"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
