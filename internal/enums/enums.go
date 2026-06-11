// Package enums mirrors the Laravel app's string-backed PHP enums, including the
// exact value->label maps used by the API transforms.
package enums

// UserRole — note the backing values are the human-readable role names exactly
// as stored in the Spatie `roles` table.
const (
	RoleAdmin          = "Admin"
	RoleAdminOperator  = "Admin Operator"
	RoleOperator       = "Operator"
	RoleAirportHandler = "Airport Handler"
	RoleMutawif        = "Mutawif"
	RoleFinance        = "Finance"
	RoleRunner         = "Runner"
	RoleSnackHandler   = "Snack Handler"
	RoleCustomer       = "Customer"
	RoleAccountant     = "Accountant"
	RoleReservation    = "Reservation"
	RoleSales          = "Sales"
)

// UserCashType (user_cashes.type)
const (
	UserCashIncome   = "d"
	UserCashExpense  = "c"
	UserCashTransfer = "t"
)

// CashCategoryType (cash_categories.type)
const (
	CashCategoryIncome   = "in"
	CashCategoryExpense  = "out"
	CashCategoryTransfer = "transfer"
	CashCategoryParent   = "parent"
)

// ExpenseGroup (cash_categories.group)
const (
	ExpenseGroupVendorPayment   = "vendor-payment"
	ExpenseGroupAirportHandling = "airport-handling"
	ExpenseGroupHotelCheckInOut = "hotel-check-in-out"
	ExpenseGroupMutawif         = "mutawif"
)

// GroupTaskStatus
const (
	GroupTaskOpen      = "open"
	GroupTaskCompleted = "completed"
	GroupTaskObsolete  = "obsolete"
)

// ServiceType
const (
	ServiceHandling        = "handling"
	ServiceVisa            = "visa"
	ServiceHotel           = "hotel"
	ServiceAirportHandling = "airport_handling"
	ServiceAdditional      = "additional"
)

var serviceTypeLabels = map[string]string{
	ServiceHandling:        "Handling",
	ServiceVisa:            "Visa",
	ServiceHotel:           "Hotel",
	ServiceAirportHandling: "Airport Handling",
	ServiceAdditional:      "Additional",
}

var groupTaskStatusLabels = map[string]string{
	GroupTaskOpen:      "Open",
	GroupTaskCompleted: "Completed",
	GroupTaskObsolete:  "Obsolete",
}

var groupTaskEventLabels = map[string]string{
	"arrival":         "Arrival",
	"hotel_check_in":  "Hotel Check-In",
	"hotel_check_out": "Hotel Check-Out",
	"ziarah":          "Ziarah",
	"train_transfer":  "Train Transfer",
	"departure":       "Departure",
}

var groupTaskTeamLabels = map[string]string{
	"airport":          "Airport Team",
	"check_in":         "Runner",
	"transport":        "Transport",
	"mutawif":          "Mutawif",
	"media":            "Media",
	"station_transfer": "Station / Transfer Team",
}

var incidentCategoryLabels = map[string]string{
	"general":   "General",
	"hotel":     "Hotel",
	"flight":    "Flight",
	"transport": "Transport",
	"pilgrim":   "Pilgrim",
	"finance":   "Finance",
	"medical":   "Medical",
	"document":  "Document",
}

var incidentSeverityLabels = map[string]string{
	"low":      "Low",
	"medium":   "Medium",
	"high":     "High",
	"critical": "Critical",
}

var incidentStatusLabels = map[string]string{
	"open":        "Open",
	"in_progress": "In Progress",
	"resolved":    "Resolved",
	"closed":      "Closed",
}

// BaggageCheckpoint (group_baggage_counts.checkpoint)
const (
	BaggageBeforeDeparture = "before_departure"
	BaggageOnArrival       = "on_arrival"
	BaggageBeforeCheckIn   = "before_check_in"
	BaggageAfterCheckOut   = "after_check_out"
	BaggageCityTransfer    = "city_transfer"
	BaggageBeforeReturn    = "before_return"
	BaggageOnReturn        = "on_return"
)

var baggageCheckpointLabels = map[string]string{
	BaggageBeforeDeparture: "Before Departure",
	BaggageOnArrival:       "On Arrival",
	BaggageBeforeCheckIn:   "Before Hotel Check-In",
	BaggageAfterCheckOut:   "After Hotel Check-Out",
	BaggageCityTransfer:    "City Transfer",
	BaggageBeforeReturn:    "Before Return Departure",
	BaggageOnReturn:        "On Return Arrival",
}

// luggageTagTints maps a luggage tag color value to its subtle background tint
// (Tailwind ~50 shade). "none"/unknown yields no tint.
var luggageTagTints = map[string]string{
	"red":    "#fee2e2",
	"orange": "#ffedd5",
	"yellow": "#fef9c3",
	"green":  "#dcfce7",
	"teal":   "#ccfbf1",
	"blue":   "#dbeafe",
	"purple": "#f3e8ff",
	"pink":   "#fce7f3",
	"gray":   "#f3f4f6",
}

// label returns the mapped label, falling back to the raw value.
func label(m map[string]string, v string) string {
	if l, ok := m[v]; ok {
		return l
	}
	return v
}

func GroupTaskStatusLabel(v string) string  { return label(groupTaskStatusLabels, v) }
func ServiceTypeLabel(v string) string      { return label(serviceTypeLabels, v) }
func GroupTaskEventLabel(v string) string   { return label(groupTaskEventLabels, v) }
func GroupTaskTeamLabel(v string) string    { return label(groupTaskTeamLabels, v) }
func IncidentCategoryLabel(v string) string { return label(incidentCategoryLabels, v) }
func IncidentSeverityLabel(v string) string { return label(incidentSeverityLabels, v) }
func IncidentStatusLabel(v string) string   { return label(incidentStatusLabels, v) }

// BaggageCheckpointLabel returns the mapped label for a checkpoint value.
func BaggageCheckpointLabel(v string) string { return label(baggageCheckpointLabels, v) }

// IsBaggageCheckpoint reports whether v is a valid checkpoint value.
func IsBaggageCheckpoint(v string) bool {
	_, ok := baggageCheckpointLabels[v]
	return ok
}

// LuggageTagTint returns the tint hex for the given color value, or empty string
// when there is no tint (mirrors PHP's nullable return).
func LuggageTagTint(v string) string { return luggageTagTints[v] }
