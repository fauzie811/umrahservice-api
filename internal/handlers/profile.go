package handlers

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProfileUpdate mirrors Api\ProfileController::update.
func (h *Handler) ProfileUpdate(c *gin.Context) {
	p := h.principal(c)
	user := p.User

	// `sometimes` semantics: only apply fields that are present in the request.
	if _, ok := c.GetPostForm("name"); ok {
		name, _ := c.GetPostForm("name")
		if strings.TrimSpace(name) == "" {
			validationError(c, map[string][]string{"name": {"The name field is required."}})
			return
		}
		user.Name = name
	}

	if _, ok := c.GetPostForm("phone"); ok {
		phone, _ := c.GetPostForm("phone")
		if phone == "" {
			user.Phone = nil
		} else {
			user.Phone = &phone
		}
	}

	if fh, err := c.FormFile("photo"); err == nil && fh != nil {
		if !isImage(fh.Filename) {
			validationError(c, map[string][]string{"photo": {"The photo field must be an image."}})
			return
		}
		content, contentType, ext, err := readUpload(fh)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not read upload."})
			return
		}
		if user.Photo != nil && *user.Photo != "" {
			_ = h.Storage.Delete(c.Request.Context(), *user.Photo)
		}
		key, err := h.Storage.Store(c.Request.Context(), "user_photos", ext, contentType, content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not store photo."})
			return
		}
		user.Photo = &key
	}

	if err := h.DB.Model(user).Select("name", "phone", "photo").Updates(map[string]interface{}{
		"name":  user.Name,
		"phone": user.Phone,
		"photo": user.Photo,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not save profile."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"user":    h.userPayload(p),
	})
}

func isImage(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp":
		return true
	}
	return false
}
