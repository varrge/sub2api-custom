package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Keep the custom catalog's OpenAI-first order, then append registered platforms
// in their historical display order. TypeSafe is not a Codex endpoint.
func apiKeyCompositeCatalogPlatforms(includeSystemOne bool) []string {
	platforms := []string{service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini}
	for _, spec := range domain.Platforms() {
		switch spec.ID {
		case service.PlatformOpenAI, service.PlatformAnthropic, service.PlatformGemini:
			continue
		case service.PlatformTypeSafe:
			if !includeSystemOne {
				continue
			}
		}
		platforms = append(platforms, spec.ID)
	}
	return platforms
}
