package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"umrahservice-api/internal/auth"
	"umrahservice-api/internal/enums"
	"umrahservice-api/internal/models"
)

type scheduleItem struct {
	data     gin.H
	dateTime string
}

// Schedule mirrors Api\ScheduleController::index.
func (h *Handler) Schedule(c *gin.Context) {
	p := h.principal(c)

	from := c.Query("from")
	to := c.Query("to")
	if from == "" {
		from = startOfWeek(time.Now()).Format("2006-01-02")
	}
	if to == "" {
		to = startOfWeek(time.Now()).AddDate(0, 0, 14).Format("2006-01-02")
	}

	airports := h.airportNames()
	var items []scheduleItem

	confirmedGroup := func(q *gorm.DB) *gorm.DB {
		return q.Where("EXISTS (SELECT 1 FROM groups g WHERE g.id = group_flights.group_id AND g.status = 'confirmed')")
	}

	// Arrivals
	{
		q := h.DB.Model(&models.GroupFlight{}).Preload("Group.Customer").
			Where("type = ?", "arrival")
		q = confirmedGroup(q)
		q = h.applyFlightRoleScopes(q, p, "to")
		q = q.Where("date_eta BETWEEN ? AND ?", from, to)

		var flights []models.GroupFlight
		q.Find(&flights)
		for i := range flights {
			f := &flights[i]
			items = append(items, h.flightItem(f, "arrival", "Arrival", f.To, f.DateETA, airports))
		}
	}

	// Departures
	{
		q := h.DB.Model(&models.GroupFlight{}).Preload("Group.Customer").
			Where("type = ?", "departure")
		q = confirmedGroup(q)
		q = h.applyFlightRoleScopes(q, p, "from")
		q = q.Where("date_etd BETWEEN ? AND ?", from, to)

		var flights []models.GroupFlight
		q.Find(&flights)
		for i := range flights {
			f := &flights[i]
			items = append(items, h.flightItem(f, "departure", "Departure", f.From, f.DateETD, airports))
		}
	}

	// Check-ins / Check-outs
	items = append(items, h.hotelItems(p, from, to, true)...)
	items = append(items, h.hotelItems(p, from, to, false)...)

	sort.SliceStable(items, func(i, j int) bool { return items[i].dateTime < items[j].dateTime })

	data := make([]gin.H, 0, len(items))
	for _, it := range items {
		data = append(data, it.data)
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) applyFlightRoleScopes(q *gorm.DB, p *auth.Principal, codeColumn string) *gorm.DB {
	if p.HasExactRoles(enums.RoleAirportHandler) {
		if len(p.VendorIDs) > 0 {
			var codes []string
			h.DB.Table("airport_handler").Where("handler_id IN ?", p.VendorIDs).Pluck("airport_code", &codes)
			if len(codes) > 0 {
				q = q.Where(codeColumn+" IN ?", codes)
			}
			q = q.Where("handler_id IN ?", p.VendorIDs)
		}
	}
	if p.HasExactRoles(enums.RoleMutawif) {
		uid := p.User.ID
		q = q.Where("EXISTS (SELECT 1 FROM groups g WHERE g.id = group_flights.group_id AND (g.mutawif_id = ? OR g.mutawif_2_id = ? OR g.mutawif_3_id = ?))", uid, uid, uid)
	}
	return q
}

func (h *Handler) flightItem(f *models.GroupFlight, typ, title string, code *string, dt *time.Time, airports map[string]string) scheduleItem {
	loc := ""
	if code != nil {
		loc = *code
		if name, ok := airports[*code]; ok {
			loc = *code + " - " + name
		} else {
			loc = *code + " - "
		}
	}
	dateTime := ""
	if dt != nil {
		dateTime = dt.Format("2006-01-02T15:04:05")
	}
	return scheduleItem{
		dateTime: dateTime,
		data: gin.H{
			"id":            typ + "-" + itoa(f.ID),
			"type":          typ,
			"group_name":    groupName(f.Group),
			"customer_name": groupCustomerName(f.Group),
			"title":         title,
			"location":      loc,
			"date_time":     dateTime,
			"total_pax":     h.flightTotalPax(f),
		},
	}
}

func (h *Handler) flightTotalPax(f *models.GroupFlight) int {
	if f.Pax != nil {
		return *f.Pax
	}
	if f.Group != nil {
		return f.Group.TotalPax()
	}
	return 0
}

func (h *Handler) hotelItems(p *auth.Principal, from, to string, checkIn bool) []scheduleItem {
	q := h.DB.Model(&models.GroupHotel{}).Preload("Group.Customer").Preload("Hotel").
		Where("EXISTS (SELECT 1 FROM groups g WHERE g.id = group_hotel.group_id AND g.status = 'confirmed')")

	if p.HasExactRoles(enums.RoleRunner) {
		q = q.Where(`EXISTS (
			SELECT 1 FROM groups g
			JOIN group_service gs ON gs.group_id = g.id
			JOIN services s ON s.id = gs.service_id
			WHERE g.id = group_hotel.group_id AND s.type IN ?
		)`, []string{enums.ServiceHotel, enums.ServiceHandling})
	}

	col := "check_in"
	if !checkIn {
		col = "check_out"
	}
	q = q.Where(col+" BETWEEN ? AND ?", from, to)

	var ghs []models.GroupHotel
	q.Find(&ghs)

	out := make([]scheduleItem, 0, len(ghs))
	for i := range ghs {
		gh := &ghs[i]
		typ, title, t, clock := "checkIn", "Check-In", gh.CheckIn, "T15:00:00"
		if !checkIn {
			typ, title, t, clock = "checkOut", "Check-Out", gh.CheckOut, "T12:00:00"
		}
		dateTime := ""
		if t != nil {
			dateTime = t.Format("2006-01-02") + clock
		}
		loc := ""
		if gh.Hotel != nil {
			loc = gh.Hotel.Fullname
		}
		var totalPax int
		if gh.Group != nil {
			totalPax = gh.Group.TotalPax()
		}
		out = append(out, scheduleItem{
			dateTime: dateTime,
			data: gin.H{
				"id":            typ + "-" + itoa(gh.ID),
				"type":          typ,
				"group_name":    groupName(gh.Group),
				"customer_name": groupCustomerName(gh.Group),
				"title":         title,
				"location":      loc,
				"date_time":     dateTime,
				"total_pax":     totalPax,
			},
		})
	}
	return out
}

func (h *Handler) airportNames() map[string]string {
	var airports []models.Airport
	h.DB.Find(&airports)
	m := make(map[string]string, len(airports))
	for _, a := range airports {
		m[a.Code] = a.Name
	}
	return m
}

// startOfWeek returns Monday 00:00 of t's week (Carbon default).
func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday
	}
	monday := t.AddDate(0, 0, -(weekday - 1))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
}
