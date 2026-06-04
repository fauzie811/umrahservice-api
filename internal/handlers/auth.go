package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/models"
)

type loginRequest struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

// Login mirrors Api\AuthController::login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	_ = c.ShouldBind(&req)

	errs := map[string][]string{}
	if strings.TrimSpace(req.Email) == "" {
		errs["email"] = []string{"The email field is required."}
	} else if !strings.Contains(req.Email, "@") {
		errs["email"] = []string{"The email field must be a valid email address."}
	}
	if req.Password == "" {
		errs["password"] = []string{"The password field is required."}
	}
	if len(errs) > 0 {
		validationError(c, errs)
		return
	}

	var user models.User
	err := h.DB.Where("email = ?", req.Email).First(&user).Error
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		validationError(c, map[string][]string{
			"email": {"The provided credentials are incorrect."},
		})
		return
	}

	token, err := auth.CreateToken(h.DB, user.ID, "api-token")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not create token."})
		return
	}

	p, err := auth.LoadPrincipal(h.DB, &user, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Server error."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  h.userPayload(p),
		"token": token,
	})
}

// Logout mirrors Api\AuthController::logout (deletes the current token).
func (h *Handler) Logout(c *gin.Context) {
	p := h.principal(c)
	if p.Token != nil {
		h.DB.Delete(&models.PersonalAccessToken{}, p.Token.ID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully."})
}

// CurrentUser mirrors the GET /user closure.
func (h *Handler) CurrentUser(c *gin.Context) {
	c.JSON(http.StatusOK, h.userPayload(h.principal(c)))
}
