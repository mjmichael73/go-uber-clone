package handler

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mjmichael73/go-uber-clone/pkg/auth"
	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/user"
	"github.com/mjmichael73/go-uber-clone/services/user-service/model"
	"github.com/mjmichael73/go-uber-clone/services/user-service/repository"
)

type UserHandler struct {
	pb.UnimplementedUserServiceServer
	repo       *repository.UserRepository
	jwtManager *auth.JWTManager
}

func NewUserHandler(repo *repository.UserRepository, jwtManager *auth.JWTManager) *UserHandler {
	return &UserHandler{
		repo:       repo,
		jwtManager: jwtManager,
	}
}

func (h *UserHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// Validate input
	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.Phone == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}

	// Check if user already exists
	existing, _ := h.repo.GetByEmail(ctx, req.Email)
	if existing != nil {
		return nil, status.Error(codes.AlreadyExists, "user with this email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	userType := "RIDER"
	if req.UserType == pb.UserType_DRIVER {
		userType = "DRIVER"
	}

	user := &model.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Phone:        req.Phone,
		UserType:     userType,
	}

	if err := h.repo.Create(ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	token, err := h.jwtManager.Generate(user.ID, user.UserType, user.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &pb.RegisterResponse{
		UserId: user.ID,
		Token:  token,
	}, nil
}

func (h *UserHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password required")
	}

	user, err := h.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, status.Error(codes.NotFound, "invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}

	token, err := h.jwtManager.Generate(user.ID, user.UserType, user.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	userType := pb.UserType_RIDER
	if user.UserType == "DRIVER" {
		userType = pb.UserType_DRIVER
	}

	return &pb.LoginResponse{
		UserId:   user.ID,
		Token:    token,
		UserType: userType,
	}, nil
}

func (h *UserHandler) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.UserProfile, error) {
	user, err := h.repo.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return userToProto(user), nil
}

func (h *UserHandler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UserProfile, error) {
	user, err := h.repo.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}

	if err := h.repo.Update(ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update profile: %v", err)
	}

	return userToProto(user), nil
}

func (h *UserHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := h.jwtManager.Verify(req.Token)
	if err != nil {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	userType := pb.UserType_RIDER
	if claims.UserType == "DRIVER" {
		userType = pb.UserType_DRIVER
	}

	return &pb.ValidateTokenResponse{
		Valid:    true,
		UserId:   claims.UserID,
		UserType: userType,
	}, nil
}

func userToProto(user *model.User) *pb.UserProfile {
	userType := pb.UserType_RIDER
	if user.UserType == "DRIVER" {
		userType = pb.UserType_DRIVER
	}

	return &pb.UserProfile{
		UserId:    user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Phone:     user.Phone,
		UserType:  userType,
		Rating:    user.Rating,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}
}

// For reference - not used in handler but part of auth flow
func getDummyExpiry() time.Time {
	return time.Now().Add(24 * time.Hour)
}
