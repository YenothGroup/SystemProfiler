package ui

import "strings"

const noGraphicsDetected = "No Graphics Detected"

func graphicsDisplay(models []string) string {
	seen := make(map[string]struct{}, len(models))
	var unique []string
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || strings.EqualFold(model, noGraphicsDetected) {
			continue
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, model)
	}
	if len(unique) == 0 {
		return noGraphicsDetected
	}
	return strings.Join(unique, "\n")
}

func graphicsNameIsGeneric(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "" || strings.Contains(name, "microsoft basic display") || name == "unknown device"
}
