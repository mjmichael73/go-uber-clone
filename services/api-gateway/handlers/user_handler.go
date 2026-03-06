package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	pb "github.com/mjmichael73/go-uber-clone/pkg/pb/user"
)

type UserHTTPHandler struct {
	userClient pb.UserServiceClient
}

func NewUserHTTPHandler(userClient pb.UserServiceClient) *UserHTTPHandler {
	return &UserHTTPHandler{userClient: userClient}
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Phone     string `json:"phone" binding:"required"`
	UserType  string `json:"user_type"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register godoc
// @Summary Register a new user
// @Description Register a new user with the given details
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /auth/register [post]
func (h *UserHTTPHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userType := pb.UserType_RIDER
	if req.UserType == "DRIVER" {
		userType = pb.UserType_DRIVER
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.Register(ctx, &pb.RegisterRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		UserType:  userType,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id": resp.UserId,
		"token":   resp.Token,
	})
}

// Login godoc
// @Summary Login a user
// @Description Login with email and password to get a JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /auth/login [post]
func (h *UserHTTPHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.Login(ctx, &pb.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":   resp.UserId,
		"token":     resp.Token,
		"user_type": resp.UserType.String(),
	})
}

// GetProfile godoc
// @Summary Get user profile
// @Description Get the profile of the currently authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /users/profile [get]
func (h *UserHTTPHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.GetProfile(ctx, &pb.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    resp.UserId,
		"email":      resp.Email,
		"first_name": resp.FirstName,
		"last_name":  resp.LastName,
		"phone":      resp.Phone,
		"user_type":  resp.UserType.String(),
		"rating":     resp.Rating,
		"created_at": resp.CreatedAt.AsTime(),
	})
}

type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Update the profile of the currently authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateProfileRequest true "Profile details"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/profile [put]
func (h *UserHTTPHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.UpdateProfile(ctx, &pb.UpdateProfileRequest{
		UserId:    userID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    resp.UserId,
		"first_name": resp.FirstName,
		"last_name":  resp.LastName,
		"phone":      resp.Phone,
	})
}
