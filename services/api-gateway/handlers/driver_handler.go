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

func (h *DriverHTTPHandler) RegisterDriver(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		LicenseNumber string `json:"license_number" binding:"required"`
		VehicleMake   string `json:"vehicle_make"`
		VehicleModel  string `json:"vehicle_model"`
		VehicleYear   string `json:"vehicle_year"`
		VehicleColor  string `json:"vehicle_color"`
		PlateNumber   string `json:"plate_number"`
		VehicleType   string `json:"vehicle_type"`
	}
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

func (h *DriverHTTPHandler) UpdateStatus(c *gin.Context) {
	var req struct {
		DriverID string `json:"driver_id" binding:"required"`
		Status   string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

func (h *DriverHTTPHandler) UpdateLocation(c *gin.Context) {
	var req struct {
		DriverID  string  `json:"driver_id" binding:"required"`
		Latitude  float64 `json:"latitude" binding:"required"`
		Longitude float64 `json:"longitude" binding:"required"`
		Heading   float64 `json:"heading"`
		Speed     float64 `json:"speed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.driverClient.UpdateLocation(ctx, &pb.UpdateLocationRequest{
		DriverId:  req.DriverID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Heading:   req.Heading,
		Speed:     req.Speed,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

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
