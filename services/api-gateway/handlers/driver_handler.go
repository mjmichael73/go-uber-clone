package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
)

type DriverHTTPHandler struct {
	driverClient pb.DriverServiceClient
}

func NewDriverHTTPHandler(driverClient pb.DriverServiceClient) *DriverHTTPHandler {
	return &DriverHTTPHandler{driverClient: driverClient}
}

type RegisterDriverRequest struct {
	LicenseNumber string `json:"license_number" binding:"required"`
	VehicleMake   string `json:"vehicle_make"`
	VehicleModel  string `json:"vehicle_model"`
	VehicleYear   string `json:"vehicle_year"`
	VehicleColor  string `json:"vehicle_color"`
	PlateNumber   string `json:"plate_number"`
	VehicleType   string `json:"vehicle_type"`
}

// RegisterDriver godoc
// @Summary Register as a driver
// @Description Register the current user as a driver
// @Tags drivers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body RegisterDriverRequest true "Driver registration details"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /drivers/register [post]
func (h *DriverHTTPHandler) RegisterDriver(c *gin.Context) {
	userID := c.GetString("user_id")

	var req RegisterDriverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vehicleType := pb.VehicleType_ECONOMY
	switch req.VehicleType {
	case "COMFORT":
		vehicleType = pb.VehicleType_COMFORT
	case "PREMIUM":
		vehicleType = pb.VehicleType_PREMIUM
	case "SUV":
		vehicleType = pb.VehicleType_SUV
	case "XL":
		vehicleType = pb.VehicleType_XL
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.driverClient.RegisterDriver(ctx, &pb.RegisterDriverRequest{
		UserId:        userID,
		LicenseNumber: req.LicenseNumber,
		Vehicle: &pb.VehicleInfo{
			Make:        req.VehicleMake,
			Model:       req.VehicleModel,
			Year:        req.VehicleYear,
			Color:       req.VehicleColor,
			PlateNumber: req.PlateNumber,
			VehicleType: vehicleType,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetDriver godoc
// @Summary Get driver details
// @Description Get details of a specific driver by ID
// @Tags drivers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Driver ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /drivers/{id} [get]
func (h *DriverHTTPHandler) GetDriver(c *gin.Context) {
	driverID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.driverClient.GetDriver(ctx, &pb.GetDriverRequest{DriverId: driverID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "driver not found"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

type UpdateStatusRequest struct {
	DriverID string `json:"driver_id"`
	Status   string `json:"status" binding:"required"`
}

// UpdateStatus godoc
// @Summary Update driver status
// @Description Update the status of a driver (AVAILABLE, BUSY, ON_RIDE, OFFLINE)
// @Tags drivers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateStatusRequest true "Status update details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /drivers/status [put]
func (h *DriverHTTPHandler) UpdateStatus(c *gin.Context) {
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If driver_id is not in body, try to get it from context (if driver is logged in)
	if req.DriverID == "" {
		// In a real app, we'd check if the user is a driver and get their driver_id
		// For now, let's see if it's passed in the context
		req.DriverID = c.GetString("driver_id")
	}

	if req.DriverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "driver_id is required"})
		return
	}

	driverStatus := pb.DriverStatus_OFFLINE
	switch req.Status {
	case "AVAILABLE":
		driverStatus = pb.DriverStatus_AVAILABLE
	case "BUSY":
		driverStatus = pb.DriverStatus_BUSY
	case "ON_RIDE":
		driverStatus = pb.DriverStatus_ON_RIDE
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.driverClient.UpdateStatus(ctx, &pb.UpdateStatusRequest{
		DriverId: req.DriverID,
		Status:   driverStatus,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

type UpdateLocationRequest struct {
	DriverID  string   `json:"driver_id"`
	Latitude  *float64 `json:"latitude" binding:"required"`
	Longitude *float64 `json:"longitude" binding:"required"`
	Heading   float64  `json:"heading"`
	Speed     float64  `json:"speed"`
}

// UpdateLocation godoc
// @Summary Update driver location
// @Description Update the current geographical location of a driver
// @Tags drivers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateLocationRequest true "Location update details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /drivers/location [post]
func (h *DriverHTTPHandler) UpdateLocation(c *gin.Context) {
	var req UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DriverID == "" {
		req.DriverID = c.GetString("driver_id")
	}

	if req.DriverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "driver_id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.driverClient.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		DriverId:  req.DriverID,
		Latitude:  *req.Latitude,
		Longitude: *req.Longitude,
		Heading:   req.Heading,
		Speed:     req.Speed,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetNearbyDrivers godoc
// @Summary Get nearby drivers
// @Description Get a list of drivers near a specific location
// @Tags drivers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param latitude query number true "Latitude"
// @Param longitude query number true "Longitude"
// @Param radius query number false "Radius in KM (default 5)"
// @Param limit query int false "Limit results (default 10)"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /drivers/nearby [get]
func (h *DriverHTTPHandler) GetNearbyDrivers(c *gin.Context) {
	lat, _ := strconv.ParseFloat(c.Query("latitude"), 64)
	lng, _ := strconv.ParseFloat(c.Query("longitude"), 64)
	radius, _ := strconv.ParseFloat(c.DefaultQuery("radius", "5"), 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.driverClient.GetNearbyDrivers(ctx, &pb.GetNearbyDriversRequest{
		Latitude:  lat,
		Longitude: lng,
		RadiusKm:  radius,
		Limit:     int32(limit),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
