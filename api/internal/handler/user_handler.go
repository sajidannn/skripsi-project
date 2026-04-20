package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/service"
	"github.com/sajidannn/pos-api/internal/validator"
)

// UserHandler handles HTTP requests for /auth and /users resources.
type UserHandler struct {
	svc *service.UserService
}

// NewUserHandler returns a new UserHandler.
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Login handles POST /auth/login.
// It does NOT require a JWT — it produces one.
func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	token, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "database error") {
			_ = c.Error(apierr.Internal(err, "database error during login"))
		} else {
			_ = c.Error(apierr.Unauthorized(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, dto.Success(dto.LoginResponse{Token: token}))
}

// Create handles POST /users.
func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	user, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create user"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toUserResponse(user)))
}

// GetByID handles GET /users/:id.
func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid user id"))
		return
	}

	user, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "user not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toUserResponse(user)))
}

// List handles GET /users.
func (h *UserHandler) List(c *gin.Context) {
	var rawQ dto.PageQuery
	if err := c.ShouldBindQuery(&rawQ); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}
	q := rawQ.Validate([]string{"id", "name", "email", "role", "created_at"}, "id")

	var f dto.UserFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid filter parameters"))
		return
	}

	if s := c.Query("date_from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			f.DateFrom = &t
		} else {
			_ = c.Error(apierr.BadRequest("date_from must be in YYYY-MM-DD format"))
			return
		}
	}
	if s := c.Query("date_to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			f.DateTo = &end
		} else {
			_ = c.Error(apierr.BadRequest("date_to must be in YYYY-MM-DD format"))
			return
		}
	}

	users, total, err := h.svc.List(c.Request.Context(), q, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list users"))
		return
	}

	resp := make([]dto.UserResponse, len(users))
	for i, u := range users {
		resp[i] = toUserResponse(&u)
	}
	c.JSON(http.StatusOK, dto.PagedOK(resp, dto.NewPageMeta(q, total)))
}

// Update handles PUT /users/:id.
func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid user id"))
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	user, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to update user"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toUserResponse(user)))
}

// Delete handles DELETE /users/:id.
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid user id"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to delete user"))
		return
	}

	c.JSON(http.StatusOK, dto.Success[any](nil))
}

// toUserResponse maps a domain model to the HTTP response DTO.
func toUserResponse(u *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}
