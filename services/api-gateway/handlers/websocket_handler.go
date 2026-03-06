package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	driverPb "github.com/mjmichael73/go-uber-clone/pkg/pb/driver"
	notifPb "github.com/mjmichael73/go-uber-clone/pkg/pb/notification"
	ridePb "github.com/mjmichael73/go-uber-clone/pkg/pb/ride"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

type WebSocketHandler struct {
	rideClient   ridePb.RideServiceClient
	driverClient driverPb.DriverServiceClient
	notifClient  notifPb.NotificationServiceClient
}

func NewWebSocketHandler(
	rideClient ridePb.RideServiceClient,
	driverClient driverPb.DriverServiceClient,
	notifClient notifPb.NotificationServiceClient,
) *WebSocketHandler {
	return &WebSocketHandler{
		rideClient:   rideClient,
		driverClient: driverClient,
		notifClient:  notifClient,
	}
}

// HandleRideUpdates streams real-time ride updates via WebSocket
func (h *WebSocketHandler) HandleRideUpdates(c *gin.Context) {
	rideID := c.Param("ride_id")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe to ride updates via gRPC streaming
	stream, err := h.rideClient.StreamRideUpdates(ctx, &ridePb.StreamRideRequest{
		RideId: rideID,
	})
	if err != nil {
		conn.WriteJSON(gin.H{"error": "failed to subscribe to ride updates"})
		return
	}

	// Read goroutine to detect client disconnect
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Forward gRPC stream to WebSocket
	for {
		update, err := stream.Recv()
		if err != nil {
			log.Printf("Stream error for ride %s: %v", rideID, err)
			break
		}

		msg := gin.H{
			"type":    "ride_update",
			"ride_id": update.RideId,
			"status":  update.Status.String(),
			"driver_location": gin.H{
				"latitude":  update.DriverLocation.GetLatitude(),
				"longitude": update.DriverLocation.GetLongitude(),
			},
			"eta_minutes": update.EtaMinutes,
			"timestamp":   update.Timestamp.AsTime(),
		}

		if err := conn.WriteJSON(msg); err != nil {
			break
		}
	}
}

// HandleDriverLocation streams driver location updates
func (h *WebSocketHandler) HandleDriverLocation(c *gin.Context) {
	driverID := c.Param("driver_id")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Bidirectional streaming with driver service
	stream, err := h.driverClient.StreamDriverLocation(ctx)
	if err != nil {
		conn.WriteJSON(gin.H{"error": "failed to connect to location stream"})
		return
	}

	// Receive location updates from the client (driver app) and forward to gRPC
	go func() {
		for {
			var locUpdate struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
				Heading   float64 `json:"heading"`
				Speed     float64 `json:"speed"`
			}
			if err := conn.ReadJSON(&locUpdate); err != nil {
				cancel()
				return
			}

			stream.Send(&driverPb.UpdateLocationRequest{
				DriverId:  driverID,
				Latitude:  locUpdate.Latitude,
				Longitude: locUpdate.Longitude,
				Heading:   locUpdate.Heading,
				Speed:     locUpdate.Speed,
			})
		}
	}()

	// Receive confirmations from gRPC and forward to WebSocket
	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}

		conn.WriteJSON(gin.H{
			"type":      "location_ack",
			"driver_id": resp.DriverId,
			"timestamp": resp.Timestamp.AsTime(),
		})
	}
}

// HandleNotifications streams real-time notifications
func (h *WebSocketHandler) HandleNotifications(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := h.notifClient.StreamNotifications(ctx, &notifPb.StreamNotificationsRequest{
		UserId: userID,
	})
	if err != nil {
		conn.WriteJSON(gin.H{"error": "failed to subscribe to notifications"})
		return
	}

	// Detect client disconnect
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Heartbeat to keep connection alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteJSON(gin.H{"type": "ping"}); err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		notif, err := stream.Recv()
		if err != nil {
			break
		}

		conn.WriteJSON(gin.H{
			"type":            "notification",
			"notification_id": notif.NotificationId,
			"title":           notif.Title,
			"body":            notif.Body,
			"notif_type":      notif.Type.String(),
			"data":            notif.Data,
			"created_at":      notif.CreatedAt.AsTime(),
		})
	}
}
