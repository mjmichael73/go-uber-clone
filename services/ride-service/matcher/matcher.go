package matcher

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	driverPb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
	notifPb "github.com/mjmichael73/go-uber-clone/pkg/pb/notification"
	"github.com/mjmichael73/go-uber-clone/services/ride-service/repository"
)

type RideMatcher struct {
	driverClient driverPb.DriverServiceClient
	notifClient  notifPb.NotificationServiceClient
	rideRepo     *repository.RideRepository
}

func NewRideMatcher(driverAddr, notifAddr string, rideRepo *repository.RideRepository) (*RideMatcher, error) {
	// Connect to Driver Service
	driverConn, err := grpc.Dial(driverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// Connect to Notification Service
	notifConn, err := grpc.Dial(notifAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &RideMatcher{
		driverClient: driverPb.NewDriverServiceClient(driverConn),
		notifClient:  notifPb.NewNotificationServiceClient(notifConn),
		rideRepo:     rideRepo,
	}, nil
}

type MatchResult struct {
	DriverID   string
	DistanceKm float64
	ETAMinutes float64
	Rating     float32
	Score      float64
}

func (m *RideMatcher) FindBestDriver(ctx context.Context, rideID string, pickupLat, pickupLng float64, vehicleType string) (*MatchResult, error) {
	// Get nearby available drivers
	resp, err := m.driverClient.GetNearbyDrivers(ctx, &driverPb.GetNearbyDriversRequest{
		Latitude:  pickupLat,
		Longitude: pickupLng,
		RadiusKm:  10.0, // 10km search radius
		Limit:     20,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Drivers) == 0 {
		return nil, nil // No drivers available
	}

	// Score and rank drivers
	var candidates []MatchResult
	for _, driver := range resp.Drivers {
		score := calculateMatchScore(driver.DistanceKm, float64(driver.Rating), driver.EtaMinutes)
		candidates = append(candidates, MatchResult{
			DriverID:   driver.DriverId,
			DistanceKm: driver.DistanceKm,
			ETAMinutes: driver.EtaMinutes,
			Rating:     driver.Rating,
			Score:      score,
		})
	}

	// Sort by score (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	bestMatch := &candidates[0]

	// Notify the matched driver
	go m.notifyDriver(bestMatch.DriverID, rideID, pickupLat, pickupLng)

	return bestMatch, nil
}

func calculateMatchScore(distanceKm float64, rating float64, etaMinutes float64) float64 {
	// Weighted scoring:
	// - Closer drivers get higher scores
	// - Higher rated drivers get bonus
	// - Lower ETA is preferred

	distanceScore := 100.0 / (1.0 + distanceKm) // Inverse distance
	ratingScore := rating * 10                  // Rating bonus
	etaScore := 50.0 / (1.0 + etaMinutes)       // Inverse ETA

	return distanceScore*0.5 + ratingScore*0.3 + etaScore*0.2
}

func (m *RideMatcher) notifyDriver(driverID, rideID string, lat, lng float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := m.notifClient.SendNotification(ctx, &notifPb.SendNotificationRequest{
		UserId:  driverID,
		Title:   "New Ride Request!",
		Body:    "A rider is requesting a ride nearby. Tap to accept.",
		Type:    notifPb.NotificationType_RIDE_REQUESTED,
		Channel: notifPb.NotificationChannel_PUSH,
		Data: map[string]string{
			"ride_id":   rideID,
			"latitude":  fmt.Sprintf("%f", lat),
			"longitude": fmt.Sprintf("%f", lng),
		},
	})
	if err != nil {
		log.Printf("Failed to notify driver %s: %v", driverID, err)
	}
}
