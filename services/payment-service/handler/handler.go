package handler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/payment"
	"github.com/mjmichael73/go-uber-clone/services/payment-service/calculator"
)

type PaymentHandler struct {
	pb.UnimplementedPaymentServiceServer
	db *sql.DB
}

func NewPaymentHandler(db *sql.DB) *PaymentHandler {
	return &PaymentHandler{db: db}
}

func (h *PaymentHandler) CalculateFare(ctx context.Context, req *pb.CalculateFareRequest) (*pb.CalculateFareResponse, error) {
	breakdown := calculator.CalculateFare(
		req.DistanceKm,
		req.DurationMinutes,
		req.VehicleType,
		req.SurgeMultiplier,
	)

	return &pb.CalculateFareResponse{
		BaseFare:     breakdown.BaseFare,
		DistanceFare: breakdown.DistanceFare,
		TimeFare:     breakdown.TimeFare,
		SurgeCharge:  breakdown.SurgeCharge,
		BookingFee:   breakdown.BookingFee,
		TotalFare:    breakdown.TotalFare,
		Currency:     breakdown.Currency,
	}, nil
}

func (h *PaymentHandler) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.PaymentResponse, error) {
	paymentID := uuid.New().String()

	query := `
		INSERT INTO payments (id, ride_id, rider_id, driver_id, amount, currency, 
		                      status, payment_method_id)
		VALUES ($1, $2, $3, $4, $5, $6, 'PENDING', $7)
		RETURNING created_at`

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	var createdAt time.Time
	err := h.db.QueryRowContext(ctx, query,
		paymentID, req.RideId, req.RiderId, req.DriverId,
		req.Amount, currency, req.PaymentMethodId,
	).Scan(&createdAt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create payment: %v", err)
	}

	return &pb.PaymentResponse{
		PaymentId:       paymentID,
		RideId:          req.RideId,
		RiderId:         req.RiderId,
		DriverId:        req.DriverId,
		Amount:          req.Amount,
		Currency:        currency,
		Status:          pb.PaymentStatus_PENDING,
		PaymentMethodId: req.PaymentMethodId,
		CreatedAt:       timestamppb.New(createdAt),
	}, nil
}

func (h *PaymentHandler) ProcessPayment(ctx context.Context, req *pb.ProcessPaymentRequest) (*pb.PaymentResponse, error) {
	// In production, integrate with Stripe/PayPal/etc.
	// This is a simplified mock implementation

	// Update payment status to PROCESSING
	_, err := h.db.ExecContext(ctx,
		`UPDATE payments SET status = 'PROCESSING' WHERE id = $1`, req.PaymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process payment: %v", err)
	}

	// Simulate payment processing
	time.Sleep(100 * time.Millisecond)

	// Mark as completed
	_, err = h.db.ExecContext(ctx,
		`UPDATE payments SET status = 'COMPLETED', processed_at = NOW() WHERE id = $1`, req.PaymentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to complete payment: %v", err)
	}

	return h.GetPayment(ctx, &pb.GetPaymentRequest{PaymentId: req.PaymentId})
}

func (h *PaymentHandler) RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest) (*pb.PaymentResponse, error) {
	_, err := h.db.ExecContext(ctx,
		`UPDATE payments SET status = 'REFUNDED', refund_amount = $2, refund_reason = $3, 
		 refunded_at = NOW() WHERE id = $1`,
		req.PaymentId, req.Amount, req.Reason)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to refund payment: %v", err)
	}

	return h.GetPayment(ctx, &pb.GetPaymentRequest{PaymentId: req.PaymentId})
}

func (h *PaymentHandler) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.PaymentResponse, error) {
	query := `
		SELECT id, ride_id, rider_id, COALESCE(driver_id::text, ''), amount, currency, 
		       status, COALESCE(payment_method_id::text, ''), created_at, processed_at
		FROM payments WHERE id = $1`

	var (
		payment     pb.PaymentResponse
		statusStr   string
		processedAt sql.NullTime
	)

	err := h.db.QueryRowContext(ctx, query, req.PaymentId).Scan(
		&payment.PaymentId, &payment.RideId, &payment.RiderId,
		&payment.DriverId, &payment.Amount, &payment.Currency,
		&statusStr, &payment.PaymentMethodId,
		&payment.CreatedAt, &processedAt,
	)
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "payment not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get payment: %v", err)
	}

	switch statusStr {
	case "PENDING":
		payment.Status = pb.PaymentStatus_PENDING
	case "PROCESSING":
		payment.Status = pb.PaymentStatus_PROCESSING
	case "COMPLETED":
		payment.Status = pb.PaymentStatus_COMPLETED
	case "FAILED":
		payment.Status = pb.PaymentStatus_FAILED
	case "REFUNDED":
		payment.Status = pb.PaymentStatus_REFUNDED
	}

	if processedAt.Valid {
		payment.ProcessedAt = timestamppb.New(processedAt.Time)
	}

	return &payment, nil
}

func (h *PaymentHandler) GetPaymentHistory(ctx context.Context, req *pb.GetPaymentHistoryRequest) (*pb.GetPaymentHistoryResponse, error) {
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var totalCount int32
	h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM payments WHERE rider_id = $1 OR driver_id = $1`,
		req.UserId).Scan(&totalCount)

	rows, err := h.db.QueryContext(ctx,
		`SELECT id FROM payments WHERE rider_id = $1 OR driver_id = $1 
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		req.UserId, pageSize, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get payment history: %v", err)
	}
	defer rows.Close()

	var payments []*pb.PaymentResponse
	for rows.Next() {
		var paymentID string
		if err := rows.Scan(&paymentID); err != nil {
			continue
		}
		payment, err := h.GetPayment(ctx, &pb.GetPaymentRequest{PaymentId: paymentID})
		if err == nil {
			payments = append(payments, payment)
		}
	}

	return &pb.GetPaymentHistoryResponse{
		Payments:   payments,
		TotalCount: totalCount,
	}, nil
}

func (h *PaymentHandler) AddPaymentMethod(ctx context.Context, req *pb.AddPaymentMethodRequest) (*pb.PaymentMethodResponse, error) {
	methodID := uuid.New().String()
	lastFour := ""
	if len(req.CardNumber) >= 4 {
		lastFour = req.CardNumber[len(req.CardNumber)-4:]
	}

	brand := detectCardBrand(req.CardNumber)

	query := `
		INSERT INTO payment_methods (id, user_id, type, last_four, brand, is_default)
		VALUES ($1, $2, $3, $4, $5, $6)`

	// Check if this is the first payment method (make it default)
	var count int
	h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_methods WHERE user_id = $1`, req.UserId).Scan(&count)
	isDefault := count == 0

	_, err := h.db.ExecContext(ctx, query, methodID, req.UserId, req.Type, lastFour, brand, isDefault)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add payment method: %v", err)
	}

	return &pb.PaymentMethodResponse{
		Id:        methodID,
		UserId:    req.UserId,
		Type:      req.Type,
		LastFour:  lastFour,
		Brand:     brand,
		IsDefault: isDefault,
	}, nil
}

func (h *PaymentHandler) GetPaymentMethods(ctx context.Context, req *pb.GetPaymentMethodsRequest) (*pb.GetPaymentMethodsResponse, error) {
	rows, err := h.db.QueryContext(ctx,
		`SELECT id, user_id, type, last_four, brand, is_default 
		 FROM payment_methods WHERE user_id = $1 ORDER BY created_at DESC`,
		req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get payment methods: %v", err)
	}
	defer rows.Close()

	var methods []*pb.PaymentMethodResponse
	for rows.Next() {
		method := &pb.PaymentMethodResponse{}
		err := rows.Scan(&method.Id, &method.UserId, &method.Type,
			&method.LastFour, &method.Brand, &method.IsDefault)
		if err != nil {
			continue
		}
		methods = append(methods, method)
	}

	return &pb.GetPaymentMethodsResponse{Methods: methods}, nil
}

func detectCardBrand(cardNumber string) string {
	if len(cardNumber) == 0 {
		return "unknown"
	}
	switch cardNumber[0] {
	case '4':
		return "Visa"
	case '5':
		return "Mastercard"
	case '3':
		return "Amex"
	case '6':
		return "Discover"
	default:
		return "unknown"
	}
}

// Unused but needed for fmt import resolution
var _ = fmt.Sprintf
