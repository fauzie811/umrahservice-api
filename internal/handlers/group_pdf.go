package handlers

import (
	"context"

	"umrahservice-api/internal/models"
)

// pifData returns the PIF PDF filename and its base64-encoded contents for the
// group show endpoint. PIF generation is delegated to the upstream Laravel app
// (mirrors GeneratePIF::run + base64_encode there), so the Go service does not
// reproduce the PIF layout. On any error this returns empty values so the rest
// of the show payload still succeeds.
func (h *Handler) pifData(group *models.Group) (name string, base64Data string) {
	name, base64Data, err := h.Laravel.PIF(context.Background(), group.ID)
	if err != nil {
		return "", ""
	}
	return name, base64Data
}

func deString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
