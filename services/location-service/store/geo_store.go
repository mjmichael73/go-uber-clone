package store

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type LocationData struct {
	EntityID  string
	Latitude  float64
	Longitude float64
	Heading   float64
	Speed     float64
	UpdatedAt time.Time
}

type GeoStore struct {
	redis *redis.Client
}

func NewGeoStore(rdb *redis.Client) *GeoStore {
	return &GeoStore{redis: rdb}
}

func (s *GeoStore) UpdateLocation(ctx context.Context, entityID, entityType string, lat, lng, heading, speed float64) error {
	geoKey := fmt.Sprintf("geo:%s", entityType)

	// Add to geospatial index
	err := s.redis.GeoAdd(ctx, geoKey, &redis.GeoLocation{
		Name:      entityID,
		Longitude: lng,
		Latitude:  lat,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to update geo location: %w", err)
	}

	// Store detailed location data
	detailKey := fmt.Sprintf("location:%s:%s", entityType, entityID)
	pipe := s.redis.Pipeline()
	pipe.HSet(ctx, detailKey, map[string]interface{}{
		"latitude":  lat,
		"longitude": lng,
		"heading":   heading,
		"speed":     speed,
		"timestamp": time.Now().Unix(),
	})
	pipe.Expire(ctx, detailKey, 10*time.Minute)

	// Store location history for route tracking
	historyKey := fmt.Sprintf("location:history:%s:%s", entityType, entityID)
	locationEntry := fmt.Sprintf("%f,%f,%d", lat, lng, time.Now().Unix())
	pipe.RPush(ctx, historyKey, locationEntry)
	pipe.LTrim(ctx, historyKey, -1000, -1) // Keep last 1000 points
	pipe.Expire(ctx, historyKey, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

func (s *GeoStore) GetLocation(ctx context.Context, entityID, entityType string) (*LocationData, error) {
	detailKey := fmt.Sprintf("location:%s:%s", entityType, entityID)

	result, err := s.redis.HGetAll(ctx, detailKey).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("location not found for %s", entityID)
	}

	lat, _ := strconv.ParseFloat(result["latitude"], 64)
	lng, _ := strconv.ParseFloat(result["longitude"], 64)
	heading, _ := strconv.ParseFloat(result["heading"], 64)
	speed, _ := strconv.ParseFloat(result["speed"], 64)
	timestamp, _ := strconv.ParseInt(result["timestamp"], 10, 64)

	return &LocationData{
		EntityID:  entityID,
		Latitude:  lat,
		Longitude: lng,
		Heading:   heading,
		Speed:     speed,
		UpdatedAt: time.Unix(timestamp, 0),
	}, nil
}

func (s *GeoStore) GetNearby(ctx context.Context, entityType string, lat, lng, radiusKm float64, limit int) ([]LocationData, error) {
	geoKey := fmt.Sprintf("geo:%s", entityType)

	results, err := s.redis.GeoRadius(ctx, geoKey, lng, lat, &redis.GeoRadiusQuery{
		Radius:    radiusKm,
		Unit:      "km",
		WithCoord: true,
		WithDist:  true,
		Count:     limit,
		Sort:      "ASC",
	}).Result()
	if err != nil {
		return nil, err
	}

	var locations []LocationData
	for _, r := range results {
		locations = append(locations, LocationData{
			EntityID:  r.Name,
			Latitude:  r.Latitude,
			Longitude: r.Longitude,
		})
	}
	return locations, nil
}

func (s *GeoStore) CalculateETA(lat1, lon1, lat2, lon2 float64) (float64, float64) {
	distanceKm := haversine(lat1, lon1, lat2, lon2)
	// Assume average speed of 30 km/h in city
	etaMinutes := (distanceKm / 30.0) * 60.0
	return etaMinutes, distanceKm
}

func (s *GeoStore) CalculateRoute(lat1, lon1, lat2, lon2 float64) (float64, float64, [][]float64) {
	distanceKm := haversine(lat1, lon1, lat2, lon2)
	durationMin := (distanceKm / 30.0) * 60.0

	// Generate simple polyline (in production, use a routing engine)
	steps := 10
	polyline := make([][]float64, steps+1)
	for i := 0; i <= steps; i++ {
		fraction := float64(i) / float64(steps)
		polyline[i] = []float64{
			lat1 + (lat2-lat1)*fraction,
			lon1 + (lon2-lon1)*fraction,
		}
	}

	return distanceKm, durationMin, polyline
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
