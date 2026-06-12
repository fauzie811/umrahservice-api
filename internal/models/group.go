package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Group maps the `groups` table.
type Group struct {
	ID            uint64         `gorm:"primaryKey"`
	Name          *string        `gorm:"column:name"`
	Number        *string        `gorm:"column:number"`
	CustomerID    *uint64        `gorm:"column:customer_id"`
	PaxAdults     *int           `gorm:"column:pax_adults"`
	PaxChildren   *int           `gorm:"column:pax_children"`
	PaxInfants    *int           `gorm:"column:pax_infants"`
	ArrivalDate   *time.Time     `gorm:"column:arrival_date"`
	DepartureDate *time.Time     `gorm:"column:departure_date"`
	Progress      *string        `gorm:"column:progress"`
	Status        string         `gorm:"column:status"`
	PeriodID      *uint64        `gorm:"column:period_id"`
	TourLeaderID  *uint64        `gorm:"column:tour_leader_id"`
	MutawifID     *uint64        `gorm:"column:mutawif_id"`
	Mutawif2ID    *uint64        `gorm:"column:mutawif_2_id"`
	Mutawif3ID    *uint64        `gorm:"column:mutawif_3_id"`
	TransportID   *uint64        `gorm:"column:transport_id"`
	MuassasahID   *uint64        `gorm:"column:muassasah_id"`
	HotelHandlers datatypes.JSON `gorm:"column:hotel_handlers"`
	Meals         datatypes.JSON `gorm:"column:meals"`
	Meta          datatypes.JSON `gorm:"column:meta"`

	Customer  *Customer `gorm:"foreignKey:CustomerID"`
	Mutawif   *User     `gorm:"foreignKey:MutawifID"`
	Mutawif2  *User     `gorm:"foreignKey:Mutawif2ID"`
	Mutawif3  *User     `gorm:"foreignKey:Mutawif3ID"`
	Muassasah *Vendor   `gorm:"foreignKey:MuassasahID"`
	Services  []Service `gorm:"many2many:group_service;foreignKey:ID;joinForeignKey:group_id;References:ID;joinReferences:service_id"`
}

func (Group) TableName() string { return "groups" }

// TotalPax mirrors Group::totalPax accessor.
func (g *Group) TotalPax() int {
	var t int
	if g.PaxAdults != nil {
		t += *g.PaxAdults
	}
	if g.PaxChildren != nil {
		t += *g.PaxChildren
	}
	if g.PaxInfants != nil {
		t += *g.PaxInfants
	}
	return t
}

// FullName mirrors Group::getFullNameAttribute ("customer.name (name)").
func (g *Group) FullName() string {
	name := ""
	if g.Customer != nil {
		name = g.Customer.Name
	}
	if g.Name != nil && *g.Name != "" {
		name += " (" + *g.Name + ")"
	}
	return name
}

// CustomerName safely returns the related customer name.
func (g *Group) CustomerName() *string {
	if g.Customer != nil {
		return &g.Customer.Name
	}
	return nil
}

// Mutawifs mirrors Group::mutawifs accessor (non-null mutawif, mutawif_2, mutawif_3).
func (g *Group) Mutawifs() []*User {
	out := []*User{}
	for _, m := range []*User{g.Mutawif, g.Mutawif2, g.Mutawif3} {
		if m != nil {
			out = append(out, m)
		}
	}
	return out
}

// Confirmed scope: status = 'confirmed'.
func Confirmed(q *gorm.DB) *gorm.DB {
	return q.Where("status = ?", "confirmed")
}

// CurrentPeriod scope. strict=false also matches NULL period_id.
func CurrentPeriod(periodID uint64, strict bool) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		if strict {
			return q.Where("period_id = ?", periodID)
		}
		return q.Where("period_id = ? OR period_id IS NULL", periodID)
	}
}

// Customer maps `customers`.
type Customer struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string `gorm:"column:name"`
}

func (Customer) TableName() string { return "customers" }

// Manasik maps `manasiks`.
type Manasik struct {
	ID      uint64         `gorm:"primaryKey"`
	GroupID uint64         `gorm:"column:group_id"`
	Name    *string        `gorm:"column:name"`
	Date    *time.Time     `gorm:"column:date"`
	Meta    datatypes.JSON `gorm:"column:meta"`
	Sort    *int           `gorm:"column:sort"`
}

func (Manasik) TableName() string { return "manasiks" }

// Itinerary maps `itineraries`. The former `location` column was renamed to
// `title` (it held the trip name); `location` is now a separate nullable place.
// The old `is_arrival` boolean was folded into event_type ('arrival').
type Itinerary struct {
	ID          uint64     `gorm:"primaryKey"`
	GroupID     uint64     `gorm:"column:group_id"`
	Date        *time.Time `gorm:"column:date"`
	City        *string    `gorm:"column:city"`
	Title       *string    `gorm:"column:title"`
	Location    *string    `gorm:"column:location"`
	Description *string    `gorm:"column:description"`
	EventType   *string    `gorm:"column:event_type"`
	Sort        *int       `gorm:"column:sort"`
}

func (Itinerary) TableName() string { return "itineraries" }

// Contact maps `contacts` (tour leaders are contacts with type=tour-leader).
type Contact struct {
	ID    uint64  `gorm:"primaryKey"`
	Name  string  `gorm:"column:name"`
	Phone *string `gorm:"column:phone"`
	Type  *string `gorm:"column:type"`
}

func (Contact) TableName() string { return "contacts" }

// Period maps `periods`.
type Period struct {
	ID   uint64  `gorm:"primaryKey"`
	Name *string `gorm:"column:name"`
}

func (Period) TableName() string { return "periods" }

// GroupData maps `group_data` (one row per group, file references).
type GroupData struct {
	ID       uint64         `gorm:"primaryKey"`
	GroupID  uint64         `gorm:"column:group_id"`
	Visa     *string        `gorm:"column:visa"`
	Ticket   *string        `gorm:"column:ticket"`
	Roomlist *string        `gorm:"column:roomlist"`
	Manifest *string        `gorm:"column:manifest"`
	Files    datatypes.JSON `gorm:"column:files"`
}

func (GroupData) TableName() string { return "group_data" }

// GroupFile maps `group_files` (one row per file per group).
type GroupFile struct {
	ID        uint64    `gorm:"primaryKey"`
	GroupID   uint64    `gorm:"column:group_id"`
	Name      string    `gorm:"column:name"`
	File      string    `gorm:"column:file"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (GroupFile) TableName() string { return "group_files" }

// GroupFlight maps `group_flights`.
type GroupFlight struct {
	ID        uint64         `gorm:"primaryKey"`
	GroupID   uint64         `gorm:"column:group_id"`
	AirlineID *uint64        `gorm:"column:airline_id"`
	Type      string         `gorm:"column:type"` // arrival | departure
	Pax       *int           `gorm:"column:pax"`
	From      *string        `gorm:"column:from"`
	To        *string        `gorm:"column:to"`
	DateETD   *time.Time     `gorm:"column:date_etd"`
	DateETA   *time.Time     `gorm:"column:date_eta"`
	HandlerID *uint64        `gorm:"column:handler_id"`
	Meta      datatypes.JSON `gorm:"column:meta"`

	Group   *Group   `gorm:"foreignKey:GroupID"`
	Airline *Airline `gorm:"foreignKey:AirlineID"`
}

func (GroupFlight) TableName() string { return "group_flights" }

// Airline maps `airlines`.
type Airline struct {
	ID   uint64  `gorm:"primaryKey"`
	Name *string `gorm:"column:name"`
}

func (Airline) TableName() string { return "airlines" }

// Airport maps `airports`.
type Airport struct {
	ID   uint64 `gorm:"primaryKey"`
	Code string `gorm:"column:code"`
	Name string `gorm:"column:name"`
	City string `gorm:"column:city"`
}

func (Airport) TableName() string { return "airports" }

// GroupHotel maps `group_hotel`.
type GroupHotel struct {
	ID       uint64         `gorm:"primaryKey"`
	GroupID  uint64         `gorm:"column:group_id"`
	HotelID  *uint64        `gorm:"column:hotel_id"`
	CheckIn  *time.Time     `gorm:"column:check_in"`
	CheckOut *time.Time     `gorm:"column:check_out"`
	BrokerID *uint64        `gorm:"column:broker_id"`
	Meta     datatypes.JSON `gorm:"column:meta"`

	Group *Group `gorm:"foreignKey:GroupID"`
	Hotel *Hotel `gorm:"foreignKey:HotelID"`
}

func (GroupHotel) TableName() string { return "group_hotel" }

// DateRange mirrors GroupHotel::dateRange ("d/m/Y – d/m/Y"), empty when missing.
func (gh *GroupHotel) DateRange() string {
	if gh.CheckIn == nil || gh.CheckOut == nil {
		return ""
	}
	return gh.CheckIn.Format("02/01/2006") + " – " + gh.CheckOut.Format("02/01/2006")
}

// Hotel maps `hotels`. `fullname` is a stored column in the table.
type Hotel struct {
	ID       uint64 `gorm:"primaryKey"`
	Name     string `gorm:"column:name"`
	City     string `gorm:"column:city"`
	Fullname string `gorm:"column:fullname"`
}

func (Hotel) TableName() string { return "hotels" }
