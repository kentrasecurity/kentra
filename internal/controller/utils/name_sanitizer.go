package utils

import (
	"fmt"
	"strings"
)

// GenerateJobName creates a sanitized job name from base name and group name
func GenerateJobName(baseName, groupName string, groupIndex int) string {
	sanitized := SanitizeName(groupName)
	if sanitized != "" {
		return fmt.Sprintf("%s-%s", baseName, sanitized)
	}
	return fmt.Sprintf("%s-group-%d", baseName, groupIndex)
}

// SanitizeName converts a name to Kubernetes-compatible format
func SanitizeName(name string) string {
	result := ""
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result += string(char)
		} else if char >= 'A' && char <= 'Z' {
			result += string(char + 32) // Convert to lowercase
		} else if char == ' ' || char == '_' || char == '-' {
			result += "-"
		}
	}

	// Limit length to 50 characters
	if len(result) > 50 {
		result = result[:50]
	}

	// Remove trailing hyphens
	result = strings.TrimRight(result, "-")
	return result
}
