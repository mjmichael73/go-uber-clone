#!/bin/bash

# Alias jq to our python script if jq is not installed
if ! command -v jq &> /dev/null; then
  jq() {
    python3 "$(dirname "$0")/fake_jq.py" "$@"
  }
fi

BASE_URL="http://localhost:8080/api/v1"
echo "=== Uber Microservices API Testing ==="

# ============================================
# 1. Register a Rider
# ============================================
echo -e "\n--- 1. Register Rider ---"
RIDER_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "rider@example.com",
    "password": "password123",
    "first_name": "John",
    "last_name": "Doe",
    "phone": "+1234567890",
    "user_type": "RIDER"
  }')
echo $RIDER_RESPONSE | jq .

RIDER_TOKEN=$(echo $RIDER_RESPONSE | jq -r '.token')
RIDER_ID=$(echo $RIDER_RESPONSE | jq -r '.user_id')

# If registration failed (e.g. user exists), login to get a fresh token
if [ "$RIDER_TOKEN" == "null" ] || [ -z "$RIDER_TOKEN" ] || [ "$RIDER_TOKEN" == "" ]; then
  echo "Registration failed or user exists, logging in..."
  LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
      "email": "rider@example.com",
      "password": "password123"
    }')
  RIDER_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.token')
  RIDER_ID=$(echo $LOGIN_RESPONSE | jq -r '.user_id')
fi

echo "Rider Token: $RIDER_TOKEN"
echo "Rider ID: $RIDER_ID"

# ============================================
# 2. Register a Driver User
# ============================================
echo -e "\n--- 2. Register Driver User ---"
DRIVER_USER_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "driver@example.com",
    "password": "password123",
    "first_name": "Jane",
    "last_name": "Smith",
    "phone": "+0987654321",
    "user_type": "DRIVER"
  }')
echo $DRIVER_USER_RESPONSE | jq .

DRIVER_TOKEN=$(echo $DRIVER_USER_RESPONSE | jq -r '.token')
DRIVER_USER_ID=$(echo $DRIVER_USER_RESPONSE | jq -r '.user_id')

# If registration failed (e.g. user exists), login to get a fresh token
if [ "$DRIVER_TOKEN" == "null" ] || [ -z "$DRIVER_TOKEN" ] || [ "$DRIVER_TOKEN" == "" ]; then
  echo "Registration failed or user exists, logging in..."
  LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
      "email": "driver@example.com",
      "password": "password123"
    }')
  DRIVER_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.token')
  DRIVER_USER_ID=$(echo $LOGIN_RESPONSE | jq -r '.user_id')
fi

# ============================================
# 3. Login
# ============================================
echo -e "\n--- 3. Login as Rider ---"
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "rider@example.com",
    "password": "password123"
  }')
echo $LOGIN_RESPONSE | jq .

# ============================================
# 4. Get User Profile
# ============================================
echo -e "\n--- 4. Get Profile ---"
curl -s -X GET "$BASE_URL/users/profile" \
  -H "Authorization: Bearer $RIDER_TOKEN" | jq .

# ============================================
# 5. Register Driver Profile
# ============================================
echo -e "\n--- 5. Register Driver Profile ---"
DRIVER_RESPONSE=$(curl -s -X POST "$BASE_URL/drivers/register" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -d '{
    "license_number": "DL-123456789",
    "vehicle_make": "Toyota",
    "vehicle_model": "Camry",
    "vehicle_year": "2023",
    "vehicle_color": "Black",
    "plate_number": "ABC-1234",
    "vehicle_type": "COMFORT"
  }')
echo $DRIVER_RESPONSE | jq .
DRIVER_ID=$(echo $DRIVER_RESPONSE | jq -r '.driver_id // .driverId // .DriverId')

# ============================================
# 6. Set Driver Status to Available
# ============================================
echo -e "\n--- 6. Set Driver Available ---"
curl -s -X PUT "$BASE_URL/drivers/status" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -d "{
    \"driver_id\": \"$DRIVER_ID\",
    \"status\": \"AVAILABLE\"
  }" | jq .

# ============================================
# 7. Update Driver Location
# ============================================
echo -e "\n--- 7. Update Driver Location ---"
curl -s -X POST "$BASE_URL/drivers/location" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -d "{
    \"driver_id\": \"$DRIVER_ID\",
    \"latitude\": 40.7128,
    \"longitude\": -74.0060,
    \"heading\": 90.0,
    \"speed\": 30.0
  }" | jq .

# ============================================
# 8. Get Ride Estimate
# ============================================
echo -e "\n--- 8. Get Ride Estimate ---"
curl -s -X POST "$BASE_URL/rides/estimate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RIDER_TOKEN" \
  -d '{
    "pickup_latitude": 40.7128,
    "pickup_longitude": -74.0060,
    "dropoff_latitude": 40.7580,
    "dropoff_longitude": -73.9855
  }' | jq .

# ============================================
# 9. Request a Ride
# ============================================
echo -e "\n--- 9. Request Ride ---"
RIDE_RESPONSE=$(curl -s -X POST "$BASE_URL/rides/request" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RIDER_TOKEN" \
  -d '{
    "pickup_latitude": 40.7128,
    "pickup_longitude": -74.0060,
    "pickup_address": "123 Wall Street, New York, NY",
    "dropoff_latitude": 40.7580,
    "dropoff_longitude": -73.9855,
    "dropoff_address": "Times Square, New York, NY",
    "vehicle_type": "COMFORT",
    "payment_method": "card_default"
  }')
echo $RIDE_RESPONSE | jq .
RIDE_ID=$(echo $RIDE_RESPONSE | jq -r '.ride_id')

# ============================================
# 10. Driver Accepts Ride
# ============================================
echo -e "\n--- 10. Accept Ride ---"
curl -s -X POST "$BASE_URL/rides/$RIDE_ID/accept" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -d "{
    \"driver_id\": \"$DRIVER_ID\"
  }" | jq .

# ============================================
# 11. Start Ride
# ============================================
echo -e "\n--- 11. Start Ride ---"
curl -s -X POST "$BASE_URL/rides/$RIDE_ID/start" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -d "{
    \"driver_id\": \"$DRIVER_ID\"
  }" | jq .

# ============================================
# 12. Complete Ride
# ============================================
echo -e "\n--- 12. Complete Ride ---"
curl -s -X POST "$BASE_URL/rides/$RIDE_ID/complete" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $DRIVER_TOKEN" \
  -d "{
    \"driver_id\": \"$DRIVER_ID\",
    \"final_latitude\": 40.7580,
    \"final_longitude\": -73.9855
  }" | jq .

# ============================================
# 13. Rate the Ride
# ============================================
echo -e "\n--- 13. Rate Ride ---"
curl -s -X POST "$BASE_URL/rides/$RIDE_ID/rate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RIDER_TOKEN" \
  -d '{
    "rating": 5.0,
    "comment": "Great driver, smooth ride!"
  }' | jq .

# ============================================
# 14. Get Ride History
# ============================================
echo -e "\n--- 14. Ride History ---"
curl -s -X GET "$BASE_URL/rides/history?page=1&page_size=10" \
  -H "Authorization: Bearer $RIDER_TOKEN" | jq .

# ============================================
# 15. Get Nearby Drivers
# ============================================
echo -e "\n--- 15. Nearby Drivers ---"
curl -s -X GET "$BASE_URL/drivers/nearby?latitude=40.7128&longitude=-74.0060&radius=10&limit=5" \
  -H "Authorization: Bearer $RIDER_TOKEN" | jq .

echo -e "\n=== Testing Complete ==="