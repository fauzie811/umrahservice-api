package auth

import (
	"gorm.io/gorm"

	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
)

// staffRoles mirrors IncidentPolicy::isInternalStaff's role list.
var staffRoles = []string{
	enums.RoleAdmin,
	enums.RoleAdminOperator,
	enums.RoleOperator,
	enums.RoleFinance,
	enums.RoleAirportHandler,
	enums.RoleMutawif,
	enums.RoleCheckInOutTeam,
	enums.RoleSnackHandler,
	enums.RoleReservation,
}

// IsInternalStaff mirrors IncidentPolicy::isInternalStaff.
func (p *Principal) IsInternalStaff() bool {
	return p.HasRole(staffRoles...) && !p.HasExactRoles(enums.RoleCustomer)
}

// CanViewGroup mirrors GroupPolicy::view.
func CanViewGroup(db *gorm.DB, p *Principal, group *models.Group) bool {
	if p.IsSuperAdmin() {
		return true
	}
	if group == nil {
		return true
	}
	if p.HasExactRoles(enums.RoleCustomer) {
		var customer struct{ UserID *uint64 }
		if group.CustomerID == nil {
			return false
		}
		db.Table("customers").Select("user_id").Where("id = ?", *group.CustomerID).Scan(&customer)
		return customer.UserID != nil && *customer.UserID == p.User.ID
	}
	if p.HasExactRoles(enums.RoleAirportHandler) {
		if len(p.VendorIDs) == 0 {
			return false
		}
		var count int64
		db.Model(&models.GroupFlight{}).
			Where("group_id = ? AND handler_id IN ?", group.ID, p.VendorIDs).
			Count(&count)
		return count > 0
	}
	return true
}

// --- Incident ---

func (p *Principal) CanViewAnyIncident() bool {
	return p.IsSuperAdmin() || p.IsInternalStaff()
}

func CanViewIncident(db *gorm.DB, p *Principal, inc *models.Incident) bool {
	if p.IsSuperAdmin() {
		return true
	}
	if !p.IsInternalStaff() {
		return false
	}
	if p.IsAdminOrOperator() {
		return true
	}
	return inc.Group == nil || CanViewGroup(db, p, inc.Group)
}

func (p *Principal) CanCreateIncident() bool {
	return p.IsSuperAdmin() || p.IsInternalStaff()
}

func CanUpdateIncident(db *gorm.DB, p *Principal, inc *models.Incident) bool {
	if p.IsSuperAdmin() {
		return true
	}
	if !p.IsInternalStaff() {
		return false
	}
	if p.HasRole(enums.RoleAdmin) {
		return CanViewIncident(db, p, inc)
	}
	return inc.ReportedByID != nil && *inc.ReportedByID == p.User.ID
}

func (p *Principal) CanUpdateIncidentAllFields() bool {
	return p.IsSuperAdmin() || p.HasRole(enums.RoleAdmin)
}

func (p *Principal) CanDeleteIncident() bool {
	return p.IsSuperAdmin() || p.HasRole(enums.RoleAdmin)
}

// CanCreateIncidentProgress mirrors IncidentProgressEntryPolicy::createForIncident.
// Admin, reporter, or assignee — provided the actor can view the incident.
func CanCreateIncidentProgress(db *gorm.DB, p *Principal, inc *models.Incident) bool {
	if !CanViewIncident(db, p, inc) {
		return false
	}
	if p.IsSuperAdmin() || p.HasRole(enums.RoleAdmin) {
		return true
	}
	if inc.ReportedByID != nil && *inc.ReportedByID == p.User.ID {
		return true
	}
	if inc.AssignedToID != nil && *inc.AssignedToID == p.User.ID {
		return true
	}
	return false
}

// CanDeleteIncidentProgress mirrors IncidentProgressEntryPolicy::delete (admin only).
func (p *Principal) CanDeleteIncidentProgress() bool {
	return p.IsSuperAdmin() || p.HasRole(enums.RoleAdmin)
}

// --- GroupTask ---

func CanViewGroupTask(db *gorm.DB, p *Principal, task *models.GroupTask) bool {
	if p.IsSuperAdmin() || p.IsAdminOrOperator() {
		return true
	}
	if !p.Can("group-tasks.view") {
		return false
	}
	return p.isAssignedTo(db, task)
}

func (p *Principal) isAssignedTo(db *gorm.DB, task *models.GroupTask) bool {
	if task.AssignedUserID != nil && *task.AssignedUserID == p.User.ID {
		return true
	}
	if task.AssignedRole == nil || *task.AssignedRole == "" {
		return false
	}
	if !p.HasRole(*task.AssignedRole) {
		return false
	}
	// Unassigned Mutawif-role tasks are only visible to mutawifs of the task's group.
	if *task.AssignedRole == enums.RoleMutawif {
		if task.AssignedUserID != nil || task.GroupID == nil {
			return false
		}
		uid := p.User.ID
		var count int64
		db.Model(&models.Group{}).
			Where("id = ?", *task.GroupID).
			Where("mutawif_id = ? OR mutawif_2_id = ? OR mutawif_3_id = ?", uid, uid, uid).
			Count(&count)
		return count > 0
	}
	return true
}

// --- Message ---

// CanViewMessageable resolves the morph parent and runs its view policy.
func CanViewMessageable(db *gorm.DB, p *Principal, morphType string, id uint64) bool {
	if p.IsSuperAdmin() {
		return true
	}
	switch morphType {
	case models.MorphGroupTask:
		var task models.GroupTask
		if err := db.First(&task, id).Error; err != nil {
			return false
		}
		return CanViewGroupTask(db, p, &task)
	case models.MorphIncident:
		var inc models.Incident
		if err := db.Preload("Group").First(&inc, id).Error; err != nil {
			return false
		}
		return CanViewIncident(db, p, &inc)
	}
	return false
}

func CanUpdateMessage(p *Principal, msg *models.Message) bool {
	if p.IsSuperAdmin() {
		return true
	}
	return msg.UserID != nil && *msg.UserID == p.User.ID
}

func CanDeleteMessage(p *Principal, msg *models.Message) bool {
	if p.IsSuperAdmin() {
		return true
	}
	return (msg.UserID != nil && *msg.UserID == p.User.ID) || p.HasRole(enums.RoleAdmin)
}
