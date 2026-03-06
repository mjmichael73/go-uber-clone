package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mjmichael73/go-uber-clone/services/ride-service/model"
)

type RideRepository struct {
	db *sql.DB
}

func NewRideRepository(db *sql.DB) *RideRepository {
	return &RideRepository{db: db}
}

func (r *RideRepository) Create(ctx context.Context, ride *model.Ride) error {
	query := `
		INSERT INTO rides (rider_id, status, pickup_latitude, pickup_longitude, pickup_address,
		                   dropoff_latitude, dropoff_longitude, dropoff_address,
		                   vehicle_type, estimated_fare, surge_multiplier, payment_method)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at`

	return r.db.QueryRowContext(ctx, query,
		ride.RiderID, ride.Status, ride.PickupLatitude, ride.PickupLongitude,
		ride.PickupAddress, ride.DropoffLatitude, ride.DropoffLongitude,
		ride.DropoffAddress, ride.VehicleType, ride.EstimatedFare,
		ride.SurgeMultiplier, ride.PaymentMethod,
	).Scan(&ride.ID, &ride.CreatedAt)
}

func (r *RideRepository) GetByID(ctx context.Context, id string) (*model.Ride, error) {
	query := `
		SELECT id, rider_id, driver_id, status, pickup_latitude, pickup_longitude,
		       pickup_address, dropoff_latitude, dropoff_longitude, dropoff_address,
		       vehicle_type, estimated_fare, actual_fare, distance_km, duration_minutes,
		       surge_multiplier, payment_method, rider_rating, driver_rating,
		       COALESCE(cancellation_reason, ''), cancelled_by,
		       created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides WHERE id = $1`

	ride := &model.Ride{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&ride.ID, &ride.RiderID, &ride.DriverID, &ride.Status,
		&ride.PickupLatitude, &ride.PickupLongitude, &ride.PickupAddress,
		&ride.DropoffLatitude, &ride.DropoffLongitude, &ride.DropoffAddress,
		&ride.VehicleType, &ride.EstimatedFare, &ride.ActualFare,
		&ride.DistanceKm, &ride.DurationMinutes, &ride.SurgeMultiplier,
		&ride.PaymentMethod, &ride.RiderRating, &ride.DriverRating,
		&ride.CancellationReason, &ride.CancelledBy,
		&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt,
		&ride.CompletedAt, &ride.CancelledAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ride not found")
	}
	return ride, err
}

func (r *RideRepository) UpdateStatus(ctx context.Context, rideID, status string) error {
	query := `UPDATE rides SET status = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, rideID, status)
	return err
}

func (r *RideRepository) AssignDriver(ctx context.Context, rideID, driverID string) error {
	query := `
		UPDATE rides 
		SET driver_id = $2, status = 'DRIVER_ACCEPTED', accepted_at = NOW()
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, rideID, driverID)
	return err
}

func (r *RideRepository) StartRide(ctx context.Context, rideID string) error {
	query := `
		UPDATE rides 
		SET status = 'IN_PROGRESS', started_at = NOW()
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, rideID)
	return err
}

func (r *RideRepository) CompleteRide(ctx context.Context, rideID string, actualFare, distanceKm, durationMin float64) error {
	query := `
		UPDATE rides 
		SET status = 'COMPLETED', actual_fare = $2, distance_km = $3,
		    duration_minutes = $4, completed_at = NOW()
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, rideID, actualFare, distanceKm, durationMin)
	return err
}

func (r *RideRepository) CancelRide(ctx context.Context, rideID, cancelledBy, reason string) error {
	query := `
		UPDATE rides 
		SET status = 'CANCELLED', cancelled_by = $2, cancellation_reason = $3,
		    cancelled_at = NOW()
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, rideID, cancelledBy, reason)
	return err
}

func (r *RideRepository) GetActiveRideForUser(ctx context.Context, userID string) (*model.Ride, error) {
	query := `
		SELECT id, rider_id, driver_id, status, pickup_latitude, pickup_longitude,
		       pickup_address, dropoff_latitude, dropoff_longitude, dropoff_address,
		       vehicle_type, estimated_fare, actual_fare, distance_km, duration_minutes,
		       surge_multiplier, payment_method, rider_rating, driver_rating,
		       COALESCE(cancellation_reason, ''), cancelled_by,
		       created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides 
		WHERE (rider_id = $1 OR driver_id = $1)
		  AND status NOT IN ('COMPLETED', 'CANCELLED')
		ORDER BY created_at DESC
		LIMIT 1`

	ride := &model.Ride{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&ride.ID, &ride.RiderID, &ride.DriverID, &ride.Status,
		&ride.PickupLatitude, &ride.PickupLongitude, &ride.PickupAddress,
		&ride.DropoffLatitude, &ride.DropoffLongitude, &ride.DropoffAddress,
		&ride.VehicleType, &ride.EstimatedFare, &ride.ActualFare,
		&ride.DistanceKm, &ride.DurationMinutes, &ride.SurgeMultiplier,
		&ride.PaymentMethod, &ride.RiderRating, &ride.DriverRating,
		&ride.CancellationReason, &ride.CancelledBy,
		&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt,
		&ride.CompletedAt, &ride.CancelledAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active ride found")
	}
	return ride, err
}

func (r *RideRepository) GetRideHistory(ctx context.Context, userID string, page, pageSize int) ([]*model.Ride, int, error) {
	// Count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM rides WHERE rider_id = $1 OR driver_id = $1`
	r.db.QueryRowContext(ctx, countQuery, userID).Scan(&totalCount)

	offset := (page - 1) * pageSize
	query := `
		SELECT id, rider_id, driver_id, status, pickup_latitude, pickup_longitude,
		       pickup_address, dropoff_latitude, dropoff_longitude, dropoff_address,
		       vehicle_type, estimated_fare, actual_fare, distance_km, duration_minutes,
		       surge_multiplier, payment_method, rider_rating, driver_rating,
		       COALESCE(cancellation_reason, ''), cancelled_by,
		       created_at, accepted_at, started_at, completed_at, cancelled_at
		FROM rides
		WHERE rider_id = $1 OR driver_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rides []*model.Ride
	for rows.Next() {
		ride := &model.Ride{}
		err := rows.Scan(
			&ride.ID, &ride.RiderID, &ride.DriverID, &ride.Status,
			&ride.PickupLatitude, &ride.PickupLongitude, &ride.PickupAddress,
			&ride.DropoffLatitude, &ride.DropoffLongitude, &ride.DropoffAddress,
			&ride.VehicleType, &ride.EstimatedFare, &ride.ActualFare,
			&ride.DistanceKm, &ride.DurationMinutes, &ride.SurgeMultiplier,
			&ride.PaymentMethod, &ride.RiderRating, &ride.DriverRating,
			&ride.CancellationReason, &ride.CancelledBy,
			&ride.CreatedAt, &ride.AcceptedAt, &ride.StartedAt,
			&ride.CompletedAt, &ride.CancelledAt,
		)
		if err != nil {
			return nil, 0, err
		}
		rides = append(rides, ride)
	}

	return rides, totalCount, nil
}

func (r *RideRepository) RateRide(ctx context.Context, rideID, userID string, rating float64, isRider bool) error {
	field := "driver_rating"
	if isRider {
		field = "rider_rating"
	}

	query := fmt.Sprintf(`UPDATE rides SET %s = $2 WHERE id = $1`, field)
	_, err := r.db.ExecContext(ctx, query, rideID, rating)
	return err
}

// Used for surge pricing
func (r *RideRepository) GetActiveRideCountInArea(ctx context.Context, lat, lng, radiusKm float64, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM rides
		WHERE status IN ('REQUESTED', 'MATCHED', 'DRIVER_ACCEPTED', 'IN_PROGRESS')
		  AND created_at >= $4
		  AND (6371 * acos(
		      cos(radians($1)) * cos(radians(pickup_latitude)) *
		      cos(radians(pickup_longitude) - radians($2)) +
		      sin(radians($1)) * sin(radians(pickup_latitude))
		  )) <= $3`

	var count int
	err := r.db.QueryRowContext(ctx, query, lat, lng, radiusKm, since).Scan(&count)
	return count, err
}
