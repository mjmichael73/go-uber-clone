package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
	"github.com/mjmichael73/go-uber-clone/services/driver-service/model"
	"github.com/mjmichael73/go-uber-clone/services/driver-service/repository"
)

type DriverHandler struct {
	pb.UnimplementedDriverServiceServer
	repo        *repository.DriverRepository
	redisClient *redis.Client
}

func NewDriverHandler(repo *repository.DriverRepository, redisClient *redis.Client) *DriverHandler {
	return &DriverHandler{
		repo:        repo,
		redisClient: redisClient,
	}
}

func (h *DriverHandler) RegisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.DriverProfile, error) {
	if req.UserId == "" || req.LicenseNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and license_number required")
	}

	// Check if driver already exists for this user
	existing, _ := h.repo.GetByUserID(ctx, req.UserId)
	if existing != nil {
		return driverToProto(existing), nil
	}

	vehicleType := "ECONOMY"
	if req.Vehicle != nil {
		vehicleType = req.Vehicle.VehicleType.String()
	}

	driver := &model.Driver{
		UserID:        req.UserId,
		LicenseNumber: req.LicenseNumber,
		VehicleType:   vehicleType,
	}

	if req.Vehicle != nil {
		driver.VehicleMake = req.Vehicle.Make
		driver.VehicleModel = req.Vehicle.Model
		driver.VehicleYear = req.Vehicle.Year
		driver.VehicleColor = req.Vehicle.Color
		driver.PlateNumber = req.Vehicle.PlateNumber
	}

	if err := h.repo.Create(ctx, driver); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register driver: %v", err)
	}

	return driverToProto(driver), nil
}

func (h *DriverHandler) GetDriver(ctx context.Context, req *pb.GetDriverRequest) (*pb.DriverProfile, error) {
	driver, err := h.repo.GetByID(ctx, req.DriverId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "driver not found")
	}
	return driverToProto(driver), nil
}

func (h *DriverHandler) UpdateStatus(ctx context.Context, req *pb.UpdateStatusRequest) (*pb.DriverProfile, error) {
	statusStr := req.Status.String()

	if err := h.repo.UpdateStatus(ctx, req.DriverId, statusStr); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update status: %v", err)
	}

	// Update Redis for real-time status tracking
	key := fmt.Sprintf("driver:status:%s", req.DriverId)
	h.redisClient.Set(ctx, key, statusStr, 24*time.Hour)

	// If going offline, remove from geospatial index
	if req.Status == pb.DriverStatus_OFFLINE {
		h.redisClient.ZRem(ctx, "drivers:locations", req.DriverId)
	}

	driver, err := h.repo.GetByID(ctx, req.DriverId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "driver not found")
	}

	return driverToProto(driver), nil
}

func (h *DriverHandler) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.UpdateLocationResponse, error) {
	// Update location in PostgreSQL
	if err := h.repo.UpdateLocation(ctx, req.DriverId, req.Latitude, req.Longitude); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update location: %v", err)
	}

	// Update location in Redis GEO index for fast nearby queries
	h.redisClient.GeoAdd(ctx, "drivers:locations", &redis.GeoLocation{
		Name:      req.DriverId,
		Longitude: req.Longitude,
		Latitude:  req.Latitude,
	})

	// Store detailed location data
	locationKey := fmt.Sprintf("driver:location:%s", req.DriverId)
	h.redisClient.HSet(ctx, locationKey, map[string]interface{}{
		"latitude":  req.Latitude,
		"longitude": req.Longitude,
		"heading":   req.Heading,
		"speed":     req.Speed,
		"timestamp": time.Now().Unix(),
	})
	h.redisClient.Expire(ctx, locationKey, 5*time.Minute)

	return &pb.UpdateLocationResponse{Success: true}, nil
}

func (h *DriverHandler) GetNearbyDrivers(ctx context.Context, req *pb.GetNearbyDriversRequest) (*pb.GetNearbyDriversResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}
	radiusKm := req.RadiusKm
	if radiusKm <= 0 {
		radiusKm = 5.0
	}

	// Try Redis GEO first for performance
	results, err := h.redisClient.GeoRadius(ctx, "drivers:locations", req.Longitude, req.Latitude, &redis.GeoRadiusQuery{
		Radius:    radiusKm,
		Unit:      "km",
		WithCoord: true,
		WithDist:  true,
		Count:     limit,
		Sort:      "ASC",
	}).Result()

	var nearbyDrivers []*pb.NearbyDriver

	if err == nil && len(results) > 0 {
		for _, result := range results {
			driverID := result.Name

			// Check if driver is available
			statusKey := fmt.Sprintf("driver:status:%s", driverID)
			driverStatus, _ := h.redisClient.Get(ctx, statusKey).Result()
			if driverStatus != "AVAILABLE" {
				continue
			}

			driver, err := h.repo.GetByID(ctx, driverID)
			if err != nil {
				continue
			}

			// Filter by vehicle type if specified
			if req.VehicleType != pb.VehicleType_ECONOMY && driver.VehicleType != req.VehicleType.String() {
				continue
			}

			etaMinutes := result.Dist / 0.5 // Rough estimate: 30 km/h average

			nearbyDrivers = append(nearbyDrivers, &pb.NearbyDriver{
				DriverId:   driverID,
				Latitude:   result.Latitude,
				Longitude:  result.Longitude,
				DistanceKm: result.Dist,
				Rating:     driver.Rating,
				Vehicle: &pb.VehicleInfo{
					Make:        driver.VehicleMake,
					Model:       driver.VehicleModel,
					Year:        driver.VehicleYear,
					Color:       driver.VehicleColor,
					PlateNumber: driver.PlateNumber,
				},
				EtaMinutes: etaMinutes,
			})
		}
	} else {
		// Fallback to PostgreSQL
		vehicleType := ""
		if req.VehicleType != pb.VehicleType_ECONOMY {
			vehicleType = req.VehicleType.String()
		}

		drivers, err := h.repo.FindNearby(ctx, req.Latitude, req.Longitude, radiusKm, vehicleType, limit)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to find nearby drivers: %v", err)
		}

		for _, driver := range drivers {
			dist := repository.Haversine(req.Latitude, req.Longitude, driver.CurrentLatitude, driver.CurrentLongitude)
			nearbyDrivers = append(nearbyDrivers, &pb.NearbyDriver{
				DriverId:   driver.ID,
				Latitude:   driver.CurrentLatitude,
				Longitude:  driver.CurrentLongitude,
				DistanceKm: dist,
				Rating:     driver.Rating,
				Vehicle: &pb.VehicleInfo{
					Make:        driver.VehicleMake,
					Model:       driver.VehicleModel,
					Year:        driver.VehicleYear,
					Color:       driver.VehicleColor,
					PlateNumber: driver.PlateNumber,
				},
				EtaMinutes: dist / 0.5,
			})
		}
	}

	return &pb.GetNearbyDriversResponse{Drivers: nearbyDrivers}, nil
}

func (h *DriverHandler) StreamDriverLocation(stream pb.DriverService_StreamDriverLocationServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Update location
		_, updateErr := h.UpdateLocation(stream.Context(), req)
		if updateErr != nil {
			log.Printf("Error updating driver location: %v", updateErr)
		}

		// Send back confirmation with timestamp
		err = stream.Send(&pb.LocationUpdate{
			DriverId:  req.DriverId,
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
			Timestamp: timestamppb.Now(),
		})
		if err != nil {
			return err
		}
	}
}

func driverToProto(driver *model.Driver) *pb.DriverProfile {
	driverStatus := pb.DriverStatus_OFFLINE
	switch driver.Status {
	case "AVAILABLE":
		driverStatus = pb.DriverStatus_AVAILABLE
	case "BUSY":
		driverStatus = pb.DriverStatus_BUSY
	case "ON_RIDE":
		driverStatus = pb.DriverStatus_ON_RIDE
	}

	return &pb.DriverProfile{
		DriverId:      driver.ID,
		UserId:        driver.UserID,
		LicenseNumber: driver.LicenseNumber,
		Vehicle: &pb.VehicleInfo{
			Make:        driver.VehicleMake,
			Model:       driver.VehicleModel,
			Year:        driver.VehicleYear,
			Color:       driver.VehicleColor,
			PlateNumber: driver.PlateNumber,
		},
		Status:           driverStatus,
		Rating:           driver.Rating,
		TotalRides:       int32(driver.TotalRides),
		CurrentLatitude:  driver.CurrentLatitude,
		CurrentLongitude: driver.CurrentLongitude,
		CreatedAt:        timestamppb.New(driver.CreatedAt),
	}
}
