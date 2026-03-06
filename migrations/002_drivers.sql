CREATE TABLE IF NOT EXISTS drivers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    license_number VARCHAR(50) UNIQUE NOT NULL,
    vehicle_make VARCHAR(50),
    vehicle_model VARCHAR(50),
    vehicle_year VARCHAR(4),
    vehicle_color VARCHAR(30),
    plate_number VARCHAR(20),
    vehicle_type VARCHAR(20) DEFAULT 'ECONOMY',
    status VARCHAR(20) DEFAULT 'OFFLINE',
    rating DECIMAL(3,2) DEFAULT 5.00,
    total_rides INT DEFAULT 0,
    total_ratings INT DEFAULT 0,
    current_latitude DOUBLE PRECISION DEFAULT 0,
    current_longitude DOUBLE PRECISION DEFAULT 0,
    is_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    );

CREATE INDEX idx_drivers_user_id ON drivers(user_id);
CREATE INDEX idx_drivers_status ON drivers(status);
CREATE INDEX idx_drivers_location ON drivers(current_latitude, current_longitude);
CREATE INDEX idx_drivers_vehicle_type ON drivers(vehicle_type);