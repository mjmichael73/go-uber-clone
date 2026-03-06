package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	driverPb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
	paymentPb "github.com/mjmichael73/go-uber-clone/pkg/pb/payment"
	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/ride"
	"github.com/mjmichael73/go-uber-clone/services/ride-service/matcher"
	"github.com/mjmichael73/go-uber-clone/services/ride-service/model"
	"github.com/mjmichael73/go-uber-clone/services/ride-service/repository"
)

type RideHandler struct {
	pb.UnimplementedRideServiceServer
	repo          *repository.RideRepository
	matcher       *matcher.RideMatcher
	driverClient  driverPb.DriverServiceClient
	paymentClient paymentPb.PaymentServiceClient
	redisClient   *redis.Client
	natsConn      *nats.Conn
}

func NewRideHandler(
	repo *repository.RideRepository,
	m *matcher.RideMatcher,
	driverClient driverPb.DriverServiceClient,
	paymentClient paymentPb.PaymentServiceClient,
	redisClient *redis.Client,
	natsConn *nats.Conn,
) *RideHandler {
	return &RideHandler{
		repo:          repo,
		matcher:       m,
		driverClient:  driverClient,
		paymentClient: paymentClient,
		redisClient:   redisClient,
		natsConn:      natsConn,
	}
}

func (h *RideHandler) RequestRide(ctx context.Context, req *pb.RideRequest) (*pb.RideResponse, error) {
	if req.RiderId == "" || req.PickupLocation == nil || req.DropoffLocation == nil {
		return nil, status.Error(codes.InvalidArgument, "rider_id, pickup and dropoff locations required")
	}

	// Calculate estimated fare
	distanceKm := haversine(
		req.PickupLocation.Latitude, req.PickupLocation.Longitude,
		req.DropoffLocation.Latitude, req.DropoffLocation.Longitude,
	)

	// Calculate surge
	surgeMultiplier := h.calculateSurge(ctx, req.PickupLocation.Latitude, req.PickupLocation.Longitude)

	// Get fare estimate from payment service
	fareResp, err := h.paymentClient.CalculateFare(ctx, &paymentPb.CalculateFareRequest{
		DistanceKm:      distanceKm,
		DurationMinutes: distanceKm * 2, // Rough estimate
		VehicleType:     req.VehicleType,
		SurgeMultiplier: surgeMultiplier,
	})
	if err != nil {
		log.Printf("Failed to calculate fare, using estimate: %v", err)
		fareResp = &paymentPb.CalculateFareResponse{TotalFare: distanceKm * 2.5}
	}

	vehicleType := req.VehicleType
	if vehicleType == "" {
		vehicleType = "ECONOMY"
	}

	ride := &model.Ride{
		RiderID:          req.RiderId,
		Status:           "REQUESTED",
		PickupLatitude:   req.PickupLocation.Latitude,
		PickupLongitude:  req.PickupLocation.Longitude,
		PickupAddress:    req.PickupLocation.Address,
		DropoffLatitude:  req.DropoffLocation.Latitude,
		DropoffLongitude: req.DropoffLocation.Longitude,
		DropoffAddress:   req.DropoffLocation.Address,
		VehicleType:      vehicleType,
		EstimatedFare:    fareResp.TotalFare,
		SurgeMultiplier:  surgeMultiplier,
		PaymentMethod:    req.PaymentMethod,
	}

	if err := h.repo.Create(ctx, ride); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create ride: %v", err)
	}

	// Find and match with best driver asynchronously
	go h.matchDriver(ride)

	// Publish ride event
	h.publishRideEvent("ride.requested", ride)

	return rideToProto(ride), nil
}

func (h *RideHandler) AcceptRide(ctx context.Context, req *pb.AcceptRideRequest) (*pb.RideResponse, error) {
	ride, err := h.repo.GetByID(ctx, req.RideId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "ride not found")
	}

	if ride.Status != "REQUESTED" && ride.Status != "MATCHED" {
		return nil, status.Error(codes.FailedPrecondition, "ride cannot be accepted in current state")
	}

	// Assign driver
	if err := h.repo.AssignDriver(ctx, req.RideId, req.DriverId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to assign driver: %v", err)
	}

	// Update driver status to ON_RIDE
	h.driverClient.UpdateStatus(ctx, &driverPb.UpdateStatusRequest{
		DriverId: req.DriverId,
		Status:   driverPb.DriverStatus_ON_RIDE,
	})

	ride.DriverID = sql.NullString{String: req.DriverId, Valid: true}
	ride.Status = "DRIVER_ACCEPTED"

	h.publishRideEvent("ride.accepted", ride)

	return rideToProto(ride), nil
}

func (h *RideHandler) StartRide(ctx context.Context, req *pb.StartRideRequest) (*pb.RideResponse, error) {
	ride, err := h.repo.GetByID(ctx, req.RideId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "ride not found")
	}

	if ride.Status != "DRIVER_ACCEPTED" && ride.Status != "DRIVER_ARRIVED" {
		return nil, status.Error(codes.FailedPrecondition, "ride cannot be started in current state")
	}

	if err := h.repo.StartRide(ctx, req.RideId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start ride: %v", err)
	}

	ride.Status = "IN_PROGRESS"
	h.publishRideEvent("ride.started", ride)

	return rideToProto(ride), nil
}

func (h *RideHandler) CompleteRide(ctx context.Context, req *pb.CompleteRideRequest) (*pb.RideResponse, error) {
	ride, err := h.repo.GetByID(ctx, req.RideId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "ride not found")
	}

	if ride.Status != "IN_PROGRESS" {
		return nil, status.Error(codes.FailedPrecondition, "ride is not in progress")
	}

	// Calculate actual distance and duration
	startTime := ride.StartedAt.Time
	duration := time.Since(startTime).Minutes()

	var actualDistance float64
	if req.FinalLocation != nil {
		actualDistance = haversine(
			ride.PickupLatitude, ride.PickupLongitude,
			req.FinalLocation.Latitude, req.FinalLocation.Longitude,
		)
	} else {
		actualDistance = haversine(
			ride.PickupLatitude, ride.PickupLongitude,
			ride.DropoffLatitude, ride.DropoffLongitude,
		)
	}

	// Calculate actual fare
	fareResp, err := h.paymentClient.CalculateFare(ctx, &paymentPb.CalculateFareRequest{
		DistanceKm:      actualDistance,
		DurationMinutes: duration,
		VehicleType:     ride.VehicleType,
		SurgeMultiplier: ride.SurgeMultiplier,
	})

	actualFare := ride.EstimatedFare
	if err == nil {
		actualFare = fareResp.TotalFare
	}

	// Complete the ride
	if err := h.repo.CompleteRide(ctx, req.RideId, actualFare, actualDistance, duration); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to complete ride: %v", err)
	}

	// Update driver status back to AVAILABLE
	if ride.DriverID.Valid {
		h.driverClient.UpdateStatus(ctx, &driverPb.UpdateStatusRequest{
			DriverId: ride.DriverID.String,
			Status:   driverPb.DriverStatus_AVAILABLE,
		})
	}

	// Create payment
	if ride.DriverID.Valid {
		go func() {
			payCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			h.paymentClient.CreatePayment(payCtx, &paymentPb.CreatePaymentRequest{
				RideId:          ride.ID,
				RiderId:         ride.RiderID,
				DriverId:        ride.DriverID.String,
				Amount:          actualFare,
				Currency:        "USD",
				PaymentMethodId: ride.PaymentMethod,
			})
		}()
	}

	ride.Status = "COMPLETED"
	ride.ActualFare = actualFare
	ride.DistanceKm = actualDistance
	ride.DurationMinutes = duration

	h.publishRideEvent("ride.completed", ride)

	return rideToProto(ride), nil
}

func (h *RideHandler) CancelRide(ctx context.Context, req *pb.CancelRideRequest) (*pb.RideResponse, error) {
	ride, err := h.repo.GetByID(ctx, req.RideId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "ride not found")
	}

	if ride.Status == "COMPLETED" || ride.Status == "CANCELLED" {
		return nil, status.Error(codes.FailedPrecondition, "ride cannot be cancelled")
	}

	if err := h.repo.CancelRide(ctx, req.RideId, req.CancelledBy, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cancel ride: %v", err)
	}

	// Free up driver if assigned
	if ride.DriverID.Valid {
		h.driverClient.UpdateStatus(ctx, &driverPb.UpdateStatusRequest{
			DriverId: ride.DriverID.String,
			Status:   driverPb.DriverStatus_AVAILABLE,
		})
	}

	ride.Status = "CANCELLED"
	h.publishRideEvent("ride.cancelled", ride)

	return rideToProto(ride), nil
}

func (h *RideHandler) GetRide(ctx context.Context, req *pb.GetRideRequest) (*pb.RideResponse, error) {
	ride, err := h.repo.GetByID(ctx, req.RideId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "ride not found")
	}
	return rideToProto(ride), nil
}

func (h *RideHandler) GetActiveRide(ctx context.Context, req *pb.GetActiveRideRequest) (*pb.RideResponse, error) {
	ride, err := h.repo.GetActiveRideForUser(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "no active ride found")
	}
	return rideToProto(ride), nil
}

func (h *RideHandler) GetRideHistory(ctx context.Context, req *pb.GetRideHistoryRequest) (*pb.GetRideHistoryResponse, error) {
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}

	rides, totalCount, err := h.repo.GetRideHistory(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get ride history: %v", err)
	}

	var rideResponses []*pb.RideResponse
	for _, ride := range rides {
		rideResponses = append(rideResponses, rideToProto(ride))
	}

	return &pb.GetRideHistoryResponse{
		Rides:      rideResponses,
		TotalCount: int32(totalCount),
	}, nil
}

func (h *RideHandler) RateRide(ctx context.Context, req *pb.RateRideRequest) (*pb.RateRideResponse, error) {
	ride, err := h.repo.GetByID(ctx, req.RideId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "ride not found")
	}

	if ride.Status != "COMPLETED" {
		return nil, status.Error(codes.FailedPrecondition, "can only rate completed rides")
	}

	isRider := req.UserId == ride.RiderID
	if err := h.repo.RateRide(ctx, req.RideId, req.UserId, float64(req.Rating), isRider); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to rate ride: %v", err)
	}

	return &pb.RateRideResponse{Success: true}, nil
}

func (h *RideHandler) EstimateRide(ctx context.Context, req *pb.EstimateRideRequest) (*pb.EstimateRideResponse, error) {
	distanceKm := haversine(
		req.PickupLocation.Latitude, req.PickupLocation.Longitude,
		req.DropoffLocation.Latitude, req.DropoffLocation.Longitude,
	)

	durationMin := distanceKm * 2 // Rough estimate

	surgeMultiplier := h.calculateSurge(ctx, req.PickupLocation.Latitude, req.PickupLocation.Longitude)

	vehicleTypes := []string{"ECONOMY", "COMFORT", "PREMIUM", "SUV", "XL"}
	var estimates []*pb.RideEstimate

	for _, vType := range vehicleTypes {
		fareResp, err := h.paymentClient.CalculateFare(ctx, &paymentPb.CalculateFareRequest{
			DistanceKm:      distanceKm,
			DurationMinutes: durationMin,
			VehicleType:     vType,
			SurgeMultiplier: surgeMultiplier,
		})
		if err != nil {
			continue
		}

		estimates = append(estimates, &pb.RideEstimate{
			VehicleType:              vType,
			EstimatedFareMin:         fareResp.TotalFare * 0.9,
			EstimatedFareMax:         fareResp.TotalFare * 1.1,
			EstimatedDurationMinutes: durationMin,
			EstimatedDistanceKm:      distanceKm,
			SurgeMultiplier:          surgeMultiplier,
		})
	}

	return &pb.EstimateRideResponse{Estimates: estimates}, nil
}

func (h *RideHandler) StreamRideUpdates(req *pb.StreamRideRequest, stream pb.RideService_StreamRideUpdatesServer) error {
	// Subscribe to NATS for ride updates
	subject := fmt.Sprintf("ride.updates.%s", req.RideId)
	sub, err := h.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		update := &pb.RideUpdate{}
		if err := protojson.Unmarshal(msg.Data, update); err != nil {
			log.Printf("Failed to unmarshal ride update: %v", err)
			return
		}
		if err := stream.Send(update); err != nil {
			log.Printf("Failed to send ride update: %v", err)
		}
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to subscribe to ride updates: %v", err)
	}
	defer sub.Unsubscribe()

	// Keep stream alive
	<-stream.Context().Done()
	return nil
}

// Helper methods

func (h *RideHandler) matchDriver(ride *model.Ride) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.matcher.FindBestDriver(ctx, ride.ID, ride.PickupLatitude, ride.PickupLongitude, ride.VehicleType)
	if err != nil {
		log.Printf("Failed to find driver for ride %s: %v", ride.ID, err)
		return
	}

	if result == nil {
		log.Printf("No drivers available for ride %s", ride.ID)
		h.repo.UpdateStatus(ctx, ride.ID, "CANCELLED")
		return
	}

	log.Printf("Matched ride %s with driver %s (distance: %.2f km, ETA: %.1f min)",
		ride.ID, result.DriverID, result.DistanceKm, result.ETAMinutes)
}

func (h *RideHandler) calculateSurge(ctx context.Context, lat, lng float64) float64 {
	// Count active rides in the area
	activeCount, _ := h.repo.GetActiveRideCountInArea(ctx, lat, lng, 3.0, time.Now().Add(-30*time.Minute))

	// Simple surge calculation based on demand
	switch {
	case activeCount > 50:
		return 2.5
	case activeCount > 30:
		return 2.0
	case activeCount > 15:
		return 1.5
	case activeCount > 8:
		return 1.25
	default:
		return 1.0
	}
}

func (h *RideHandler) publishRideEvent(subject string, ride *model.Ride) {
	if h.natsConn == nil {
		return
	}

	rideProto := rideToProto(ride)
	data, err := protojson.Marshal(rideProto)
	if err != nil {
		log.Printf("Failed to marshal ride event: %v", err)
		return
	}

	h.natsConn.Publish(subject, data)

	// Also publish to ride-specific channel for streaming
	rideSubject := fmt.Sprintf("ride.updates.%s", ride.ID)
	update := &pb.RideUpdate{
		RideId: ride.ID,
		Status: rideProto.Status,
		DriverLocation: &pb.Location{
			Latitude:  ride.PickupLatitude,
			Longitude: ride.PickupLongitude,
		},
		Timestamp: timestamppb.Now(),
	}
	updateData, _ := protojson.Marshal(update)
	h.natsConn.Publish(rideSubject, updateData)
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func rideToProto(ride *model.Ride) *pb.RideResponse {
	rideStatus := pb.RideStatus_REQUESTED
	switch ride.Status {
	case "MATCHED":
		rideStatus = pb.RideStatus_MATCHED
	case "DRIVER_ACCEPTED":
		rideStatus = pb.RideStatus_DRIVER_ACCEPTED
	case "DRIVER_ARRIVING":
		rideStatus = pb.RideStatus_DRIVER_ARRIVING
	case "DRIVER_ARRIVED":
		rideStatus = pb.RideStatus_DRIVER_ARRIVED
	case "IN_PROGRESS":
		rideStatus = pb.RideStatus_IN_PROGRESS
	case "COMPLETED":
		rideStatus = pb.RideStatus_COMPLETED
	case "CANCELLED":
		rideStatus = pb.RideStatus_CANCELLED
	}

	resp := &pb.RideResponse{
		RideId:  ride.ID,
		RiderId: ride.RiderID,
		Status:  rideStatus,
		PickupLocation: &pb.Location{
			Latitude:  ride.PickupLatitude,
			Longitude: ride.PickupLongitude,
			Address:   ride.PickupAddress,
		},
		DropoffLocation: &pb.Location{
			Latitude:  ride.DropoffLatitude,
			Longitude: ride.DropoffLongitude,
			Address:   ride.DropoffAddress,
		},
		VehicleType:     ride.VehicleType,
		EstimatedFare:   ride.EstimatedFare,
		ActualFare:      ride.ActualFare,
		DistanceKm:      ride.DistanceKm,
		DurationMinutes: ride.DurationMinutes,
		SurgeMultiplier: ride.SurgeMultiplier,
		CreatedAt:       timestamppb.New(ride.CreatedAt),
		PaymentMethod:   ride.PaymentMethod,
	}

	if ride.DriverID.Valid {
		resp.DriverId = ride.DriverID.String
	}
	if ride.StartedAt.Valid {
		resp.StartedAt = timestamppb.New(ride.StartedAt.Time)
	}
	if ride.CompletedAt.Valid {
		resp.CompletedAt = timestamppb.New(ride.CompletedAt.Time)
	}
	if ride.RiderRating.Valid {
		resp.RiderRating = float32(ride.RiderRating.Float64)
	}
	if ride.DriverRating.Valid {
		resp.DriverRating = float32(ride.DriverRating.Float64)
	}

	return resp
}
