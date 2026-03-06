package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"github.com/mjmichael73/go-uber-clone/services/driver-service/model"
)

type DriverRepository struct {
	db *sql.DB
}

func NewDriverRepository(db *sql.DB) *DriverRepository {
	return &DriverRepository{db: db}
}

func (r *DriverRepository) Create(ctx context.Context, driver *model.Driver) error {
	query := `
		INSERT INTO drivers (user_id, license_number, vehicle_make, vehicle_model,
		                     vehicle_year, vehicle_color, plate_number, vehicle_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, status, rating, total_rides, created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		driver.UserID, driver.LicenseNumber, driver.VehicleMake,
		driver.VehicleModel, driver.VehicleYear, driver.VehicleColor,
		driver.PlateNumber, driver.VehicleType,
	).Scan(&driver.ID, &driver.Status, &driver.Rating,
		&driver.TotalRides, &driver.CreatedAt, &driver.UpdatedAt)
}

func (r *DriverRepository) GetByID(ctx context.Context, id string) (*model.Driver, error) {
	query := `
		SELECT id, user_id, license_number, vehicle_make, vehicle_model,
		       vehicle_year, vehicle_color, plate_number, vehicle_type,
		       status, rating, total_rides, total_ratings, 
		       current_latitude, current_longitude,
		       is_verified, created_at, updated_at
		FROM drivers WHERE id = $1`

	driver := &model.Driver{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&driver.ID, &driver.UserID, &driver.LicenseNumber,
		&driver.VehicleMake, &driver.VehicleModel, &driver.VehicleYear,
		&driver.VehicleColor, &driver.PlateNumber, &driver.VehicleType,
		&driver.Status, &driver.Rating, &driver.TotalRides,
		&driver.TotalRatings, &driver.CurrentLatitude, &driver.CurrentLongitude,
		&driver.IsVerified, &driver.CreatedAt, &driver.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("driver not found")
	}
	return driver, err
}

func (r *DriverRepository) GetByUserID(ctx context.Context, userID string) (*model.Driver, error) {
	query := `
		SELECT id, user_id, license_number, vehicle_make, vehicle_model,
		       vehicle_year, vehicle_color, plate_number, vehicle_type,
		       status, rating, total_rides, total_ratings,
		       current_latitude, current_longitude,
		       is_verified, created_at, updated_at
		FROM drivers WHERE user_id = $1`

	driver := &model.Driver{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&driver.ID, &driver.UserID, &driver.LicenseNumber,
		&driver.VehicleMake, &driver.VehicleModel, &driver.VehicleYear,
		&driver.VehicleColor, &driver.PlateNumber, &driver.VehicleType,
		&driver.Status, &driver.Rating, &driver.TotalRides,
		&driver.TotalRatings, &driver.CurrentLatitude, &driver.CurrentLongitude,
		&driver.IsVerified, &driver.CreatedAt, &driver.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("driver not found")
	}
	return driver, err
}

func (r *DriverRepository) UpdateStatus(ctx context.Context, driverID, status string) error {
	query := `UPDATE drivers SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, driverID, status)
	return err
}

func (r *DriverRepository) UpdateLocation(ctx context.Context, driverID string, lat, lng float64) error {
	query := `
		UPDATE drivers 
		SET current_latitude = $2, current_longitude = $3, updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, driverID, lat, lng)
	return err
}

func (r *DriverRepository) FindNearby(ctx context.Context, lat, lng, radiusKm float64, vehicleType string, limit int) ([]*model.Driver, error) {
	// Using Haversine formula in SQL for geospatial query
	query := `
		SELECT id, user_id, license_number, vehicle_make, vehicle_model,
		       vehicle_year, vehicle_color, plate_number, vehicle_type,
		       status, rating, total_rides, total_ratings,
		       current_latitude, current_longitude,
		       is_verified, created_at, updated_at,
		       (6371 * acos(
		           cos(radians($1)) * cos(radians(current_latitude)) *
		           cos(radians(current_longitude) - radians($2)) +
		           sin(radians($1)) * sin(radians(current_latitude))
		       )) AS distance_km
		FROM drivers
		WHERE status = 'AVAILABLE'
		  AND is_verified = true
		  AND ($4 = '' OR vehicle_type = $4)
		  AND current_latitude BETWEEN $1 - ($3/111.0) AND $1 + ($3/111.0)
		  AND current_longitude BETWEEN $2 - ($3/(111.0 * cos(radians($1)))) AND $2 + ($3/(111.0 * cos(radians($1))))
		HAVING (6371 * acos(
		    cos(radians($1)) * cos(radians(current_latitude)) *
		    cos(radians(current_longitude) - radians($2)) +
		    sin(radians($1)) * sin(radians(current_latitude))
		)) <= $3
		ORDER BY distance_km ASC
		LIMIT $5`

	rows, err := r.db.QueryContext(ctx, query, lat, lng, radiusKm, vehicleType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drivers []*model.Driver
	for rows.Next() {
		driver := &model.Driver{}
		var distanceKm float64
		err := rows.Scan(
			&driver.ID, &driver.UserID, &driver.LicenseNumber,
			&driver.VehicleMake, &driver.VehicleModel, &driver.VehicleYear,
			&driver.VehicleColor, &driver.PlateNumber, &driver.VehicleType,
			&driver.Status, &driver.Rating, &driver.TotalRides,
			&driver.TotalRatings, &driver.CurrentLatitude, &driver.CurrentLongitude,
			&driver.IsVerified, &driver.CreatedAt, &driver.UpdatedAt,
			&distanceKm,
		)
		if err != nil {
			return nil, err
		}
		drivers = append(drivers, driver)
	}

	return drivers, nil
}

func (r *DriverRepository) GetByIDs(ctx context.Context, ids []string) ([]*model.Driver, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, user_id, license_number, vehicle_make, vehicle_model,
		       vehicle_year, vehicle_color, plate_number, vehicle_type,
		       status, rating, total_rides, total_ratings,
		       current_latitude, current_longitude,
		       is_verified, created_at, updated_at
		FROM drivers WHERE id = ANY($1)`

	rows, err := r.db.QueryContext(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drivers []*model.Driver
	for rows.Next() {
		driver := &model.Driver{}
		err := rows.Scan(
			&driver.ID, &driver.UserID, &driver.LicenseNumber,
			&driver.VehicleMake, &driver.VehicleModel, &driver.VehicleYear,
			&driver.VehicleColor, &driver.PlateNumber, &driver.VehicleType,
			&driver.Status, &driver.Rating, &driver.TotalRides,
			&driver.TotalRatings, &driver.CurrentLatitude, &driver.CurrentLongitude,
			&driver.IsVerified, &driver.CreatedAt, &driver.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		drivers = append(drivers, driver)
	}
	return drivers, nil
}

// Haversine distance calculation helper
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // Earth's radius in km
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
