package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"umrahservice-api/internal/models"
	"umrahservice-api/internal/pdf"
)

// pifData returns the PIF PDF filename and its base64-encoded contents for the
// group show endpoint (mirrors GeneratePIF::run + base64_encode).
//
// In local env with PIF_LOCAL_PATH set, a fixed sample PDF is returned (the
// Laravel app does the same with a hardcoded sample). Otherwise the PIF is
// rendered via Gotenberg. On any rendering error this returns empty values so
// the rest of the show payload still succeeds.
func (h *Handler) pifData(group *models.Group) (name string, base64Data string) {
	filename := pifFilename(group)

	if h.Cfg.IsLocal() && h.Cfg.PIFLocalPath != "" {
		if content, err := os.ReadFile(h.Cfg.PIFLocalPath); err == nil {
			return filepath.Base(h.Cfg.PIFLocalPath), base64.StdEncoding.EncodeToString(content)
		}
	}

	html, err := pdf.Render(h.buildPIF(group))
	if err != nil {
		return "", ""
	}
	content, err := h.PDF.ConvertHTML(context.Background(), html, filename)
	if err != nil {
		return "", ""
	}
	return filename + ".pdf", base64.StdEncoding.EncodeToString(content)
}

func pifFilename(group *models.Group) string {
	customer := ""
	if group.Customer != nil {
		customer = group.Customer.Name
	}
	gname := ""
	if group.Name != nil {
		gname = *group.Name
	}
	return fmt.Sprintf("PIF #%d - %s (%s)", group.ID, customer, gname)
}

// buildPIF assembles the PIF view-model from the group and its relations.
func (h *Handler) buildPIF(group *models.Group) pdf.PIF {
	out := pdf.PIF{
		CustomerName: deString(group.CustomerName()),
		TotalPax:     group.TotalPax(),
		PaxAdults:    intOr(group.PaxAdults),
		PaxChildren:  intOr(group.PaxChildren),
		PaxInfants:   intOr(group.PaxInfants),
	}
	if group.Name != nil {
		out.GroupName = *group.Name
	}
	if group.ArrivalDate != nil {
		out.ArrivalDate = group.ArrivalDate.Format("2006-01-02")
	}

	for _, m := range group.Mutawifs() {
		out.Mutawifs = append(out.Mutawifs, pdf.Person{Name: m.Name, Phone: deString(m.Phone)})
	}
	for _, tl := range h.tourLeaders(group) {
		out.TourLeaders = append(out.TourLeaders, pdf.Person{
			Name:  fmt.Sprintf("%v", tl["name"]),
			Phone: ginString(tl["phone"]),
		})
	}
	for _, hh := range h.hotelHandlersByCity(group) {
		out.Handlers = append(out.Handlers, pdf.Handler{City: hh.City, Name: hh.Name, Phone: hh.Phone})
	}

	// Flights.
	var flights []models.GroupFlight
	h.DB.Where("group_id = ?", group.ID).Find(&flights)
	for i := range flights {
		f := &flights[i]
		dt := ""
		t := f.DateETD
		if f.Type == "arrival" {
			t = f.DateETA
		}
		if t != nil {
			dt = t.Format("2006-01-02 15:04")
		}
		out.Flights = append(out.Flights, pdf.Flight{
			Type:     f.Type,
			From:     deString(f.From),
			To:       deString(f.To),
			DateTime: dt,
			Pax:      h.flightTotalPax(f),
		})
	}

	// Hotels.
	var ghs []models.GroupHotel
	h.DB.Preload("Hotel").Where("group_id = ?", group.ID).Find(&ghs)
	for i := range ghs {
		gh := &ghs[i]
		stay := pdf.HotelStay{}
		if gh.Hotel != nil {
			stay.City = gh.Hotel.City
			stay.Name = gh.Hotel.Name
		}
		if gh.CheckIn != nil {
			stay.CheckIn = gh.CheckIn.Format("2006-01-02")
		}
		if gh.CheckOut != nil {
			stay.CheckOut = gh.CheckOut.Format("2006-01-02")
		}
		out.Hotels = append(out.Hotels, stay)
	}

	// Manasik.
	var manasiks []models.Manasik
	h.DB.Where("group_id = ?", group.ID).Order("sort").Find(&manasiks)
	for _, m := range manasiks {
		date := ""
		if m.Date != nil {
			date = m.Date.Format("2006-01-02 15:04")
		}
		out.Manasiks = append(out.Manasiks, pdf.Manasik{Name: deString(m.Name), Date: date})
	}

	// Itinerary.
	var itineraries []models.Itinerary
	h.DB.Where("group_id = ?", group.ID).Order("sort").Find(&itineraries)
	for _, it := range itineraries {
		date := ""
		if it.Date != nil {
			date = it.Date.Format("2006-01-02")
		}
		out.Itineraries = append(out.Itineraries, pdf.ItineraryRow{
			Date:        date,
			City:        deString(it.City),
			Location:    deString(it.Location),
			Description: deString(it.Description),
		})
	}

	// Notes from meta.notes (array of strings).
	var meta map[string]interface{}
	decodeJSON(group.Meta, &meta)
	if notes, ok := meta["notes"].([]interface{}); ok {
		for _, n := range notes {
			if s, ok := n.(string); ok && s != "" {
				out.Notes = append(out.Notes, s)
			}
		}
	}

	sort.SliceStable(out.Flights, func(i, j int) bool { return out.Flights[i].DateTime < out.Flights[j].DateTime })
	return out
}

func deString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intOr(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func ginString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
