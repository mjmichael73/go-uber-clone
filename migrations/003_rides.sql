CREATE TABLE IF NOT EXISTS rides (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rider_id UUID NOT NULL REFERENCES users(id),
    driver_id UUID REFERENCES drivers(id),
    status VARCHAR(20) NOT NULL DEFAULT 'REQUESTED',
    pickup_latitude DOUBLE PRECISION NOT NULL,
    pickup_longitude DOUBLE PRECISION NOT NULL,
    pickup_address TEXT,
    dropoff_latitude DOUBLE PRECISION NOT NULL,
    dropoff_longitude DOUBLE PRECISION NOT NULL,
    dropoff_address TEXT,
    vehicle_type VARCHAR(20) DEFAULT 'ECONOMY',
    estimated_fare DECIMAL(10,2),
    actual_fare DECIMAL(10,2),
    distance_km DECIMAL(10,2),
    duration_minutes DECIMAL(10,2),
    surge_multiplier DECIMAL(4,2) DEFAULT 1.00,
    payment_method VARCHAR(50),
    rider_rating DECIMAL(3,2),
    driver_rating DECIMAL(3,2),
    rider_comment TEXT,
    driver_comment TEXT,
    cancellation_reason TEXT,
    cancelled_by UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    accepted_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    cancelled_at TIMESTAMP WITH TIME ZONE
                             );

CREATE INDEX idx_rides_rider_id ON rides(rider_id);
CREATE INDEX idx_rides_driver_id ON rides(driver_id);
CREATE INDEX idx_rides_status ON rides(status);
CREATE INDEX idx_rides_created_at ON rides(created_at);