package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/ride"
)

type RideHTTPHandler struct {
	rideClient pb.RideServiceClient
}

func NewRideHTTPHandler(rideClient pb.RideServiceClient) *RideHTTPHandler {
	return &RideHTTPHandler{rideClient: rideClient}
}

type RideRequest struct {
	PickupLat     float64 `json:"pickup_latitude" binding:"required"`
	PickupLng     float64 `json:"pickup_longitude" binding:"required"`
	PickupAddr    string  `json:"pickup_address"`
	DropoffLat    float64 `json:"dropoff_latitude" binding:"required"`
	DropoffLng    float64 `json:"dropoff_longitude" binding:"required"`
	DropoffAddr   string  `json:"dropoff_address"`
	VehicleType   string  `json:"vehicle_type"`
	PaymentMethod string  `json:"payment_method"`
}

// RequestRide godoc
// @Summary Request a ride
// @Description Request a new ride with pickup and dropoff locations
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RideRequest true "Ride request details"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/request [post]
func (h *RideHTTPHandler) RequestRide(c *gin.Context) {
	userID := c.GetString("user_id")

	var req RideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.rideClient.RequestRide(ctx, &pb.RideRequest{
		RiderId: userID,
		PickupLocation: &pb.Location{
			Latitude:  req.PickupLat,
			Longitude: req.PickupLng,
			Address:   req.PickupAddr,
		},
		DropoffLocation: &pb.Location{
			Latitude:  req.DropoffLat,
			Longitude: req.DropoffLng,
			Address:   req.DropoffAddr,
		},
		VehicleType:   req.VehicleType,
		PaymentMethod: req.PaymentMethod,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rideResponseToJSON(resp))
}

type AcceptRideRequest struct {
	DriverID string `json:"driver_id" binding:"required"`
}

// AcceptRide godoc
// @Summary Accept a ride request
// @Description Accept a ride request by a driver
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Ride ID"
// @Param request body AcceptRideRequest true "Accept ride details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/{id}/accept [post]
func (h *RideHTTPHandler) AcceptRide(c *gin.Context) {
	rideID := c.Param("id")
	var req AcceptRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.rideClient.AcceptRide(ctx, &pb.AcceptRideRequest{
		RideId:   rideID,
		DriverId: req.DriverID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rideResponseToJSON(resp))
}

type StartRideRequest struct {
	DriverID string `json:"driver_id" binding:"required"`
}

// StartRide godoc
// @Summary Start a ride
// @Description Mark a ride as started by the driver
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Ride ID"
// @Param request body StartRideRequest true "Start ride details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/{id}/start [post]
func (h *RideHTTPHandler) StartRide(c *gin.Context) {
	rideID := c.Param("id")
	var req StartRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.rideClient.StartRide(ctx, &pb.StartRideRequest{
		RideId:   rideID,
		DriverId: req.DriverID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rideResponseToJSON(resp))
}

type CompleteRideRequest struct {
	DriverID       string  `json:"driver_id" binding:"required"`
	FinalLatitude  float64 `json:"final_latitude"`
	FinalLongitude float64 `json:"final_longitude"`
}

// CompleteRide godoc
// @Summary Complete a ride
// @Description Mark a ride as completed and calculate final fare
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Ride ID"
// @Param request body CompleteRideRequest true "Complete ride details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/{id}/complete [post]
func (h *RideHTTPHandler) CompleteRide(c *gin.Context) {
	rideID := c.Param("id")
	var req CompleteRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.rideClient.CompleteRide(ctx, &pb.CompleteRideRequest{
		RideId:   rideID,
		DriverId: req.DriverID,
		FinalLocation: &pb.Location{
			Latitude:  req.FinalLatitude,
			Longitude: req.FinalLongitude,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rideResponseToJSON(resp))
}

type CancelRideRequest struct {
	Reason string `json:"reason"`
}

// CancelRide godoc
// @Summary Cancel a ride
// @Description Cancel an ongoing or requested ride
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Ride ID"
// @Param request body CancelRideRequest true "Cancel ride details"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/{id}/cancel [post]
func (h *RideHTTPHandler) CancelRide(c *gin.Context) {
	rideID := c.Param("id")
	userID := c.GetString("user_id")

	var req CancelRideRequest
	c.ShouldBindJSON(&req)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.rideClient.CancelRide(ctx, &pb.CancelRideRequest{
		RideId:      rideID,
		CancelledBy: userID,
		Reason:      req.Reason,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rideResponseToJSON(resp))
}

// GetRide godoc
// @Summary Get ride details
// @Description Get details of a specific ride by ID
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Ride ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /rides/{id} [get]
func (h *RideHTTPHandler) GetRide(c *gin.Context) {
	rideID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.rideClient.GetRide(ctx, &pb.GetRideRequest{RideId: rideID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ride not found"})
		return
	}

	c.JSON(http.StatusOK, rideResponseToJSON(resp))
}

// GetActiveRide godoc
// @Summary Get active ride
// @Description Get the currently active ride for the authenticated user
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /rides/active [get]
func (h *RideHTTPHandler) GetActiveRide(c *gin.Context) {
	userID := c.GetString("user_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.rideClient.GetActiveRide(ctx, &pb.GetActiveRideRequest{UserId: userID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active ride"})
		return
	}

	c.JSON(http.StatusOK, rideResponseToJSON(resp))
}

// GetRideHistory godoc
// @Summary Get ride history
// @Description Get the ride history for the authenticated user with pagination
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/history [get]
func (h *RideHTTPHandler) GetRideHistory(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.rideClient.GetRideHistory(ctx, &pb.GetRideHistoryRequest{
		UserId:   userID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rides []gin.H
	for _, ride := range resp.Rides {
		rides = append(rides, rideResponseToJSON(ride))
	}

	c.JSON(http.StatusOK, gin.H{
		"rides":       rides,
		"total_count": resp.TotalCount,
	})
}

type RateRideRequest struct {
	Rating  float32 `json:"rating" binding:"required,min=1,max=5"`
	Comment string  `json:"comment"`
}

// RateRide godoc
// @Summary Rate a ride
// @Description Submit a rating and comment for a completed ride
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Ride ID"
// @Param request body RateRideRequest true "Rating details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/{id}/rate [post]
func (h *RideHTTPHandler) RateRide(c *gin.Context) {
	rideID := c.Param("id")
	userID := c.GetString("user_id")

	var req RateRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	_, err := h.rideClient.RateRide(ctx, &pb.RateRideRequest{
		RideId:  rideID,
		UserId:  userID,
		Rating:  req.Rating,
		Comment: req.Comment,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "rating submitted"})
}

type EstimateRideRequest struct {
	PickupLat  float64 `json:"pickup_latitude" binding:"required"`
	PickupLng  float64 `json:"pickup_longitude" binding:"required"`
	DropoffLat float64 `json:"dropoff_latitude" binding:"required"`
	DropoffLng float64 `json:"dropoff_longitude" binding:"required"`
}

// EstimateRide godoc
// @Summary Estimate ride fare
// @Description Get fare estimates for different vehicle types between two locations
// @Tags rides
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body EstimateRideRequest true "Estimation details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /rides/estimate [post]
func (h *RideHTTPHandler) EstimateRide(c *gin.Context) {
	var req EstimateRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.rideClient.EstimateRide(ctx, &pb.EstimateRideRequest{
		PickupLocation:  &pb.Location{Latitude: req.PickupLat, Longitude: req.PickupLng},
		DropoffLocation: &pb.Location{Latitude: req.DropoffLat, Longitude: req.DropoffLng},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var estimates []gin.H
	for _, est := range resp.Estimates {
		estimates = append(estimates, gin.H{
			"vehicle_type":       est.VehicleType,
			"estimated_fare_min": est.EstimatedFareMin,
			"estimated_fare_max": est.EstimatedFareMax,
			"duration_minutes":   est.EstimatedDurationMinutes,
			"distance_km":        est.EstimatedDistanceKm,
			"surge_multiplier":   est.SurgeMultiplier,
		})
	}

	c.JSON(http.StatusOK, gin.H{"estimates": estimates})
}

func rideResponseToJSON(resp *pb.RideResponse) gin.H {
	result := gin.H{
		"ride_id":          resp.RideId,
		"rider_id":         resp.RiderId,
		"driver_id":        resp.DriverId,
		"status":           resp.Status.String(),
		"vehicle_type":     resp.VehicleType,
		"estimated_fare":   resp.EstimatedFare,
		"actual_fare":      resp.ActualFare,
		"distance_km":      resp.DistanceKm,
		"duration_minutes": resp.DurationMinutes,
		"surge_multiplier": resp.SurgeMultiplier,
		"payment_method":   resp.PaymentMethod,
	}

	if resp.PickupLocation != nil {
		result["pickup_location"] = gin.H{
			"latitude":  resp.PickupLocation.Latitude,
			"longitude": resp.PickupLocation.Longitude,
			"address":   resp.PickupLocation.Address,
		}
	}

	if resp.DropoffLocation != nil {
		result["dropoff_location"] = gin.H{
			"latitude":  resp.DropoffLocation.Latitude,
			"longitude": resp.DropoffLocation.Longitude,
			"address":   resp.DropoffLocation.Address,
		}
	}

	if resp.CreatedAt != nil {
		result["created_at"] = resp.CreatedAt.AsTime()
	}
	if resp.StartedAt != nil {
		result["started_at"] = resp.StartedAt.AsTime()
	}
	if resp.CompletedAt != nil {
		result["completed_at"] = resp.CompletedAt.AsTime()
	}

	return result
}
