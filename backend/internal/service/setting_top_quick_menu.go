package service

import (
	"encoding/json"
	"fmt"
)

const MaxTopQuickMenuItems = 3

var topQuickMenuItemIDs = map[string]struct{}{
	"image_generation": {},
	"batch_image":      {},
	"model_plaza":      {},
	"support_tickets":  {},
	"api_keys":         {},
	"usage":            {},
}

func ValidateTopQuickMenuItems(items []string) error {
	if len(items) > MaxTopQuickMenuItems {
		return fmt.Errorf("too many top quick menu items (max %d)", MaxTopQuickMenuItems)
	}
	seen := make(map[string]struct{}, len(items))
	for _, id := range items {
		if _, ok := topQuickMenuItemIDs[id]; !ok {
			return fmt.Errorf("invalid top quick menu item: %s", id)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate top quick menu item: %s", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

// ParseTopQuickMenuItems keeps valid IDs in their stored order and safely
// ignores malformed legacy values.
func ParseTopQuickMenuItems(raw string) []string {
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []string{}
	}
	result := make([]string, 0, min(len(items), MaxTopQuickMenuItems))
	seen := make(map[string]struct{}, len(items))
	for _, id := range items {
		if _, ok := topQuickMenuItemIDs[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
		if len(result) == MaxTopQuickMenuItems {
			break
		}
	}
	return result
}
