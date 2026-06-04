package auth

import (
	"fmt"

	"gorm.io/gorm"

	"umrahservice-api/internal/models"
	"umrahservice-api/internal/support"
)

// CreateToken issues a Sanctum-compatible personal access token for a user and
// returns the plaintext token ("{id}|{secret}").
func CreateToken(db *gorm.DB, userID uint64, name string) (string, error) {
	secret := support.RandomToken()

	token := models.PersonalAccessToken{
		TokenableType: models.TokenableUser,
		TokenableID:   userID,
		Name:          name,
		Token:         support.HashToken(secret),
		Abilities:     strPtr(`["*"]`),
	}
	if err := db.Create(&token).Error; err != nil {
		return "", err
	}

	return fmt.Sprintf("%d|%s", token.ID, secret), nil
}

func strPtr(s string) *string { return &s }
