package auth

import (
	"gorm.io/gorm"

	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
)

// morphUser is the morph-map alias for the User model (enforceMorphMap).
const morphUser = "user"

// Principal is the authenticated user plus their resolved Spatie roles,
// permissions and vendor ids, loaded once per request.
type Principal struct {
	User        *models.User
	Token       *models.PersonalAccessToken
	Roles       []string
	Permissions []string
	VendorIDs   []uint64
}

// LoadPrincipal resolves roles, permissions and vendor ids for a user.
func LoadPrincipal(db *gorm.DB, user *models.User, token *models.PersonalAccessToken) (*Principal, error) {
	p := &Principal{User: user, Token: token}

	if err := db.Table("roles AS r").
		Joins("JOIN model_has_roles mhr ON mhr.role_id = r.id").
		Where("mhr.model_type = ? AND mhr.model_id = ?", morphUser, user.ID).
		Pluck("r.name", &p.Roles).Error; err != nil {
		return nil, err
	}

	// Permissions via roles + direct permissions.
	if err := db.Raw(`
		SELECT p.name FROM permissions p
		JOIN role_has_permissions rhp ON rhp.permission_id = p.id
		JOIN model_has_roles mhr ON mhr.role_id = rhp.role_id
		WHERE mhr.model_type = ? AND mhr.model_id = ?
		UNION
		SELECT p.name FROM permissions p
		JOIN model_has_permissions mhp ON mhp.permission_id = p.id
		WHERE mhp.model_type = ? AND mhp.model_id = ?
	`, morphUser, user.ID, morphUser, user.ID).Scan(&p.Permissions).Error; err != nil {
		return nil, err
	}

	if err := db.Table("user_vendor").
		Where("user_id = ?", user.ID).
		Pluck("vendor_id", &p.VendorIDs).Error; err != nil {
		return nil, err
	}

	return p, nil
}

// HasRole reports whether the user has any of the given roles.
func (p *Principal) HasRole(names ...string) bool {
	for _, want := range names {
		for _, have := range p.Roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

// HasExactRoles mirrors Spatie hasExactRoles: the user has exactly the given
// set of roles (same count, all present).
func (p *Principal) HasExactRoles(names ...string) bool {
	if len(p.Roles) != len(names) {
		return false
	}
	for _, want := range names {
		if !p.HasRole(want) {
			return false
		}
	}
	return true
}

// Can reports whether the user has the given permission (direct or via role).
func (p *Principal) Can(permission string) bool {
	for _, have := range p.Permissions {
		if have == permission {
			return true
		}
	}
	return false
}

// IsSuperAdmin mirrors Gate::before (user id == 1 bypasses all gates).
func (p *Principal) IsSuperAdmin() bool {
	return p.User != nil && p.User.ID == 1
}

// IsAdminOrOperator mirrors User::isAdminOrOperator. Admins and operators may
// view every group task and incident regardless of assignment.
func (p *Principal) IsAdminOrOperator() bool {
	return p.HasRole(enums.RoleAdmin, enums.RoleAdminOperator, enums.RoleOperator)
}
