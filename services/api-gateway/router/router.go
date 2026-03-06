package router

import (
	"github.com/gin-gonic/gin"

	"github.com/mjmichael73/go-uber-clone/pkg/auth"
	"github.com/mjmichael73/go-uber-clone/services/api-gateway/handlers"
)

func SetupRouter(
	userHandler *handlers.UserHTTPHandler,
	driverHandler *handlers.DriverHTTPHandler,
	rideHandler *handlers.RideHTTPHandler,
	wsHandler *handlers.WebSocketHandler,
	jwtManager *auth.JWTManager,
) *gin.Engine {
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Public routes
	public := r.Group("/api/v1")
	{
		public.POST("/auth/register", userHandler.Register)
		public.POST("/auth/login", userHandler.Login)
	}

	// Protected routes
	protected := r.Group("/api/v1")
	protected.Use(AuthMiddleware(jwtManager))
	{
		// User routes
		protected.GET("/users/profile", userHandler.GetProfile)
		protected.PUT("/users/profile", userHandler.UpdateProfile)

		// Driver routes
		protected.POST("/drivers/register", driverHandler.RegisterDriver)
		protected.GET("/drivers/:id", driverHandler.GetDriver)
		protected.PUT("/drivers/status", driverHandler.UpdateStatus)
		protected.POST("/drivers/location", driverHandler.UpdateLocation)
		protected.GET("/drivers/nearby", driverHandler.GetNearbyDrivers)

		// Ride routes
		protected.POST("/rides/request", rideHandler.RequestRide)
		protected.POST("/rides/:id/accept", rideHandler.AcceptRide)
		protected.POST("/rides/:id/start", rideHandler.StartRide)
		protected.POST("/rides/:id/complete", rideHandler.CompleteRide)
		protected.POST("/rides/:id/cancel", rideHandler.CancelRide)
		protected.GET("/rides/:id", rideHandler.GetRide)
		protected.GET("/rides/active", rideHandler.GetActiveRide)
		protected.GET("/rides/history", rideHandler.GetRideHistory)
		protected.POST("/rides/:id/rate", rideHandler.RateRide)
		protected.POST("/rides/estimate", rideHandler.EstimateRide)
	}

	// WebSocket routes
	r.GET("/ws/ride/:ride_id", wsHandler.HandleRideUpdates)
	r.GET("/ws/driver/:driver_id/location", wsHandler.HandleDriverLocation)
	r.GET("/ws/notifications", wsHandler.HandleNotifications)

	return r
}

func AuthMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			c.JSON(401, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}

		claims, err := jwtManager.Verify(tokenStr)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_type", claims.UserType)
		c.Set("email", claims.Email)
		c.Next()
	}
}
