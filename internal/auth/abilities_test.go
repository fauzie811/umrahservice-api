package auth

import (
	"testing"

	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
)

func TestHasExactRoles(t *testing.T) {
	p := &Principal{Roles: []string{enums.RoleMutawif}}
	if !p.HasExactRoles(enums.RoleMutawif) {
		t.Fatal("single matching role should be exact")
	}
	if p.HasExactRoles(enums.RoleAdmin) {
		t.Fatal("single non-matching role should not be exact")
	}

	p2 := &Principal{Roles: []string{enums.RoleMutawif, enums.RoleAdmin}}
	if p2.HasExactRoles(enums.RoleMutawif) {
		t.Fatal("two roles cannot be exactly one role")
	}
}

func TestHasRoleAndCan(t *testing.T) {
	p := &Principal{
		Roles:       []string{enums.RoleFinance},
		Permissions: []string{"groups.updateData"},
	}
	if !p.HasRole(enums.RoleAdmin, enums.RoleFinance) {
		t.Fatal("HasRole should match any provided role")
	}
	if !p.Can("groups.updateData") {
		t.Fatal("Can should report granted permission")
	}
	if p.Can("groups.delete") {
		t.Fatal("Can should reject ungranted permission")
	}
}

func TestIsInternalStaff(t *testing.T) {
	staff := &Principal{User: &models.User{ID: 5}, Roles: []string{enums.RoleCheckInOutTeam}}
	if !staff.IsInternalStaff() {
		t.Fatal("CheckInOutTeam is internal staff")
	}
	customer := &Principal{User: &models.User{ID: 6}, Roles: []string{enums.RoleCustomer}}
	if customer.IsInternalStaff() {
		t.Fatal("a sole Customer is not internal staff")
	}
}

func TestSuperAdminBypass(t *testing.T) {
	p := &Principal{User: &models.User{ID: 1}}
	if !p.IsSuperAdmin() {
		t.Fatal("user id 1 is super admin")
	}
}
