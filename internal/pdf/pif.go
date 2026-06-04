package pdf

import (
	"bytes"
	"html/template"
)

// PIF is the view-model for the group information sheet. It is a functional
// port of pif.blade.php (key sections), not a pixel-identical reproduction.
type PIF struct {
	GroupName    string
	CustomerName string
	ArrivalDate  string
	TotalPax     int
	PaxAdults    int
	PaxChildren  int
	PaxInfants   int

	Mutawifs    []Person
	TourLeaders []Person
	Handlers    []Handler
	Flights     []Flight
	Hotels      []HotelStay
	Manasiks    []Manasik
	Itineraries []ItineraryRow
	Notes       []string
}

type Person struct{ Name, Phone string }
type Handler struct{ City, Name, Phone string }
type Flight struct {
	Type, Number, From, To, DateTime string
	Pax                              int
}
type HotelStay struct{ City, Name, CheckIn, CheckOut, Rooms, Meal string }
type Manasik struct{ Name, Date string }
type ItineraryRow struct{ Date, City, Location, Description string }

var pifTemplate = template.Must(template.New("pif").Parse(pifHTML))

// Render produces the PIF HTML for Gotenberg.
func Render(data PIF) (string, error) {
	var buf bytes.Buffer
	if err := pifTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const pifHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  @page { size: A4; margin: 18mm 14mm; }
  * { font-family: -apple-system, Arial, sans-serif; box-sizing: border-box; }
  body { color: #1f2937; font-size: 11px; }
  h1 { font-size: 18px; margin: 0 0 2px; }
  h2 { font-size: 13px; margin: 16px 0 6px; border-bottom: 2px solid #1e3a8a; color: #1e3a8a; padding-bottom: 2px; }
  .muted { color: #6b7280; }
  table { width: 100%; border-collapse: collapse; margin-top: 4px; }
  th, td { text-align: left; padding: 4px 6px; border: 1px solid #e5e7eb; vertical-align: top; }
  th { background: #f3f4f6; font-size: 10px; text-transform: uppercase; letter-spacing: .03em; }
  .header { display: flex; justify-content: space-between; align-items: flex-start; }
  .pill { display: inline-block; background: #1e3a8a; color: #fff; border-radius: 4px; padding: 2px 8px; font-size: 11px; }
  ul { margin: 4px 0; padding-left: 18px; }
</style>
</head>
<body>
  <div class="header">
    <div>
      <h1>{{.CustomerName}}</h1>
      <div class="muted">{{.GroupName}}</div>
    </div>
    <div style="text-align:right">
      <div class="pill">Arrival: {{.ArrivalDate}}</div>
      <div class="muted" style="margin-top:4px">
        Pax: {{.TotalPax}} (A{{.PaxAdults}} / C{{.PaxChildren}} / I{{.PaxInfants}})
      </div>
    </div>
  </div>

  {{if .Mutawifs}}
  <h2>Mutawifs</h2>
  <table><tr><th>Name</th><th>Phone</th></tr>
  {{range .Mutawifs}}<tr><td>{{.Name}}</td><td>{{.Phone}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .TourLeaders}}
  <h2>Tour Leaders</h2>
  <table><tr><th>Name</th><th>Phone</th></tr>
  {{range .TourLeaders}}<tr><td>{{.Name}}</td><td>{{.Phone}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Flights}}
  <h2>Flights</h2>
  <table><tr><th>Type</th><th>Flight</th><th>From</th><th>To</th><th>Date / Time</th><th>Pax</th></tr>
  {{range .Flights}}<tr><td>{{.Type}}</td><td>{{.Number}}</td><td>{{.From}}</td><td>{{.To}}</td><td>{{.DateTime}}</td><td>{{.Pax}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Hotels}}
  <h2>Hotels</h2>
  <table><tr><th>City</th><th>Hotel</th><th>Check-In</th><th>Check-Out</th><th>Rooms</th><th>Meal</th></tr>
  {{range .Hotels}}<tr><td>{{.City}}</td><td>{{.Name}}</td><td>{{.CheckIn}}</td><td>{{.CheckOut}}</td><td>{{.Rooms}}</td><td>{{.Meal}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Handlers}}
  <h2>Handlers</h2>
  <table><tr><th>City</th><th>Name</th><th>Phone</th></tr>
  {{range .Handlers}}<tr><td>{{.City}}</td><td>{{.Name}}</td><td>{{.Phone}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Manasiks}}
  <h2>Manasik</h2>
  <table><tr><th>Name</th><th>Date</th></tr>
  {{range .Manasiks}}<tr><td>{{.Name}}</td><td>{{.Date}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Itineraries}}
  <h2>Itinerary</h2>
  <table><tr><th>Date</th><th>City</th><th>Location</th><th>Description</th></tr>
  {{range .Itineraries}}<tr><td>{{.Date}}</td><td>{{.City}}</td><td>{{.Location}}</td><td>{{.Description}}</td></tr>{{end}}
  </table>
  {{end}}

  {{if .Notes}}
  <h2>Notes</h2>
  <ul>{{range .Notes}}<li>{{.}}</li>{{end}}</ul>
  {{end}}
</body>
</html>`
