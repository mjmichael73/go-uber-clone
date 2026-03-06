# Install wscat: npm install -g wscat

# Stream ride updates
wscat -c "ws://localhost:8080/ws/ride/RIDE_ID_HERE"

# Stream driver location
wscat -c "ws://localhost:8080/ws/driver/DRIVER_ID_HERE/location"
# Then send: {"latitude": 40.7128, "longitude": -74.0060, "heading": 90, "speed": 30}

# Stream notifications
wscat -c "ws://localhost:8080/ws/notifications?user_id=USER_ID_HERE"