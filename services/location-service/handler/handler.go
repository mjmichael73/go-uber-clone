package handler

import (
	"context"
	"io"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/location"
	"github.com/mjmichael73/go-uber-clone/services/location-service/store"
)

type LocationHandler struct {
	pb.UnimplementedLocationServiceServer
	geoStore *store.GeoStore
}

func NewLocationHandler(geoStore *store.GeoStore) *LocationHandler {
	return &LocationHandler{geoStore: geoStore}
}

func (h *LocationHandler) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.UpdateLocationResponse, error) {
	entityType := req.EntityType
	if entityType == "" {
		entityType = "driver"
	}

	err := h.geoStore.UpdateLocation(ctx, req.EntityId, entityType, req.Latitude, req.Longitude, req.Heading, req.Speed)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update location: %v", err)
	}

	return &pb.UpdateLocationResponse{Success: true}, nil
}

func (h *LocationHandler) GetLocation(ctx context.Context, req *pb.GetLocationRequest) (*pb.LocationResponse, error) {
	loc, err := h.geoStore.GetLocation(ctx, req.EntityId, "driver")
	if err != nil {
		return nil, status.Error(codes.NotFound, "location not found")
	}

	return &pb.LocationResponse{
		EntityId:    loc.EntityID,
		Latitude:    loc.Latitude,
		Longitude:   loc.Longitude,
		Heading:     loc.Heading,
		Speed:       loc.Speed,
		LastUpdated: timestamppb.New(loc.UpdatedAt),
	}, nil
}

func (h *LocationHandler) GetNearbyDrivers(ctx context.Context, req *pb.GetNearbyRequest) (*pb.GetNearbyResponse, error) {
	entityType := req.EntityType
	if entityType == "" {
		entityType = "driver"
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	locations, err := h.geoStore.GetNearby(ctx, entityType, req.Latitude, req.Longitude, req.RadiusKm, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get nearby entities: %v", err)
	}

	var entities []*pb.NearbyEntity
	for _, loc := range locations {
		entities = append(entities, &pb.NearbyEntity{
			EntityId:   loc.EntityID,
			Latitude:   loc.Latitude,
			Longitude:  loc.Longitude,
			DistanceKm: store.Haversine(req.Latitude, req.Longitude, loc.Latitude, loc.Longitude),
		})
	}

	return &pb.GetNearbyResponse{Entities: entities}, nil
}

func (h *LocationHandler) GetRoute(ctx context.Context, req *pb.GetRouteRequest) (*pb.GetRouteResponse, error) {
	distanceKm, durationMin, polyline := h.geoStore.CalculateRoute(
		req.Origin.Latitude, req.Origin.Longitude,
		req.Destination.Latitude, req.Destination.Longitude,
	)

	var coords []*pb.Coordinate
	for _, point := range polyline {
		coords = append(coords, &pb.Coordinate{
			Latitude:  point[0],
			Longitude: point[1],
		})
	}

	return &pb.GetRouteResponse{
		DistanceKm:      distanceKm,
		DurationMinutes: durationMin,
		Polyline:        coords,
	}, nil
}

func (h *LocationHandler) CalculateETA(ctx context.Context, req *pb.CalculateETARequest) (*pb.CalculateETAResponse, error) {
	etaMin, distKm := h.geoStore.CalculateETA(
		req.Origin.Latitude, req.Origin.Longitude,
		req.Destination.Latitude, req.Destination.Longitude,
	)

	return &pb.CalculateETAResponse{
		EtaMinutes: etaMin,
		DistanceKm: distKm,
	}, nil
}

func (h *LocationHandler) StreamLocation(stream pb.LocationService_StreamLocationServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		entityType := req.EntityType
		if entityType == "" {
			entityType = "driver"
		}

		// Update location
		if err := h.geoStore.UpdateLocation(stream.Context(), req.EntityId, entityType, req.Latitude, req.Longitude, req.Heading, req.Speed); err != nil {
			log.Printf("Error updating streamed location: %v", err)
			continue
		}

		// Send back the current location
		if err := stream.Send(&pb.LocationResponse{
			EntityId:    req.EntityId,
			Latitude:    req.Latitude,
			Longitude:   req.Longitude,
			Heading:     req.Heading,
			Speed:       req.Speed,
			LastUpdated: timestamppb.Now(),
		}); err != nil {
			return err
		}
	}
}

// Need to add context import
