// Package appinfo holds product-wide constants.
package appinfo

// Name is the product name shown in the window title and UI.
const Name = "Torrent"

// ID identifies the application for single-instance locking.
const ID = "com.aristarhucolov.torrent"

// Version is overridden at build time with -ldflags "-X".
var Version = "1.0.0"

// Link is an external page the UI is allowed to open.
type Link struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DonationLinks is the complete allowlist of external pages. The frontend can
// only ask to open a link by ID, never by arbitrary URL.
var DonationLinks = []Link{
	{ID: "buymeacoffee", Name: "Buy Me a Coffee", URL: "https://buymeacoffee.com/aristarh.ucolov"},
	{ID: "kofi", Name: "Ko-fi", URL: "https://ko-fi.com/aristarhucolov"},
	{ID: "donationalerts", Name: "DonationAlerts", URL: "https://www.donationalerts.com/r/aristarh_ucolov"},
}

// LinkByID returns an allowlisted link.
func LinkByID(id string) (Link, bool) {
	for _, l := range DonationLinks {
		if l.ID == id {
			return l, true
		}
	}
	return Link{}, false
}
