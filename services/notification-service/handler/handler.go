package handler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/notification"
)

type NotificationHandler struct {
	pb.UnimplementedNotificationServiceServer
	redisClient *redis.Client
	natsConn    *nats.Conn
	subscribers map[string][]chan *pb.NotificationEvent
	mu          sync.RWMutex
}

func NewNotificationHandler(redisClient *redis.Client, natsConn *nats.Conn) *NotificationHandler {
	return &NotificationHandler{
		redisClient: redisClient,
		natsConn:    natsConn,
		subscribers: make(map[string][]chan *pb.NotificationEvent),
	}
}

func (h *NotificationHandler) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	notificationID := uuid.New().String()

	notification := &pb.NotificationEvent{
		NotificationId: notificationID,
		UserId:         req.UserId,
		Title:          req.Title,
		Body:           req.Body,
		Type:           req.Type,
		Data:           req.Data,
		Read:           false,
		CreatedAt:      timestamppb.Now(),
	}

	// Store in Redis
	data, err := protojson.Marshal(notification)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal notification: %v", err)
	}

	// Add to user's notification list
	key := fmt.Sprintf("notifications:%s", req.UserId)
	h.redisClient.LPush(ctx, key, data)
	h.redisClient.LTrim(ctx, key, 0, 99) // Keep last 100 notifications
	h.redisClient.Expire(ctx, key, 30*24*time.Hour)

	// Publish to NATS for real-time delivery
	if h.natsConn != nil {
		subject := fmt.Sprintf("notifications.%s", req.UserId)
		h.natsConn.Publish(subject, data)
	}

	// Send to streaming subscribers
	h.notifySubscribers(req.UserId, notification)

	// Handle different channels
	switch req.Channel {
	case pb.NotificationChannel_PUSH:
		go h.sendPushNotification(req.UserId, req.Title, req.Body)
	case pb.NotificationChannel_SMS:
		go h.sendSMS(req.UserId, req.Body)
	case pb.NotificationChannel_EMAIL:
		go h.sendEmail(req.UserId, req.Title, req.Body)
	}

	log.Printf("Notification sent to user %s: %s", req.UserId, req.Title)

	return &pb.SendNotificationResponse{
		NotificationId: notificationID,
		Success:        true,
	}, nil
}

func (h *NotificationHandler) SendBulkNotification(ctx context.Context, req *pb.SendBulkNotificationRequest) (*pb.SendBulkNotificationResponse, error) {
	var sentCount, failedCount int32

	for _, userID := range req.UserIds {
		_, err := h.SendNotification(ctx, &pb.SendNotificationRequest{
			UserId:  userID,
			Title:   req.Title,
			Body:    req.Body,
			Type:    req.Type,
			Data:    req.Data,
			Channel: pb.NotificationChannel_IN_APP,
		})
		if err != nil {
			failedCount++
		} else {
			sentCount++
		}
	}

	return &pb.SendBulkNotificationResponse{
		SentCount:   sentCount,
		FailedCount: failedCount,
	}, nil
}

func (h *NotificationHandler) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.GetNotificationsResponse, error) {
	key := fmt.Sprintf("notifications:%s", req.UserId)

	page := int64(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int64(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	stop := start + pageSize - 1

	results, err := h.redisClient.LRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notifications: %v", err)
	}

	totalCount, _ := h.redisClient.LLen(ctx, key).Result()

	var notifications []*pb.NotificationEvent
	for _, data := range results {
		notif := &pb.NotificationEvent{}
		if err := protojson.Unmarshal([]byte(data), notif); err != nil {
			continue
		}
		notifications = append(notifications, notif)
	}

	return &pb.GetNotificationsResponse{
		Notifications: notifications,
		TotalCount:    int32(totalCount),
	}, nil
}

func (h *NotificationHandler) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pb.MarkAsReadResponse, error) {
	// Mark notification as read
	readKey := fmt.Sprintf("notification:read:%s", req.NotificationId)
	h.redisClient.Set(ctx, readKey, "true", 30*24*time.Hour)

	return &pb.MarkAsReadResponse{Success: true}, nil
}

func (h *NotificationHandler) StreamNotifications(req *pb.StreamNotificationsRequest, stream pb.NotificationService_StreamNotificationsServer) error {
	ch := make(chan *pb.NotificationEvent, 100)

	// Register subscriber
	h.mu.Lock()
	h.subscribers[req.UserId] = append(h.subscribers[req.UserId], ch)
	h.mu.Unlock()

	// Clean up on exit
	defer func() {
		h.mu.Lock()
		channels := h.subscribers[req.UserId]
		for i, c := range channels {
			if c == ch {
				h.subscribers[req.UserId] = append(channels[:i], channels[i+1:]...)
				break
			}
		}
		h.mu.Unlock()
		close(ch)
	}()

	// Also subscribe via NATS if available
	if h.natsConn != nil {
		subject := fmt.Sprintf("notifications.%s", req.UserId)
		sub, err := h.natsConn.Subscribe(subject, func(msg *nats.Msg) {
			notif := &pb.NotificationEvent{}
			if err := protojson.Unmarshal(msg.Data, notif); err == nil {
				select {
				case ch <- notif:
				default:
					// Channel full, skip
				}
			}
		})
		if err == nil {
			defer sub.Unsubscribe()
		}
	}

	for {
		select {
		case notif := <-ch:
			if err := stream.Send(notif); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (h *NotificationHandler) notifySubscribers(userID string, notification *pb.NotificationEvent) {
	h.mu.RLock()
	channels := h.subscribers[userID]
	h.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- notification:
		default:
		}
	}
}

// Mock implementations for notification channels
func (h *NotificationHandler) sendPushNotification(userID, title, body string) {
	log.Printf("[PUSH] To: %s | Title: %s | Body: %s", userID, title, body)
	// In production: integrate with FCM/APNs
}

func (h *NotificationHandler) sendSMS(userID, message string) {
	log.Printf("[SMS] To: %s | Message: %s", userID, message)
	// In production: integrate with Twilio/SNS
}

func (h *NotificationHandler) sendEmail(userID, subject, body string) {
	log.Printf("[EMAIL] To: %s | Subject: %s", userID, subject)
	// In production: integrate with SendGrid/SES
}
