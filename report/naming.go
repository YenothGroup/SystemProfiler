package report

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var invalidChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)

func Sanitize(input string) string {
	cleaned := invalidChars.ReplaceAllString(input, "_")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "Unknown"
	}
	return cleaned
}

// GenerateFilename builds: '$system-model - $system-serial-number -$date-time.pdf'
func GenerateFilename(model, serial string, t time.Time) string {
	safeModel := Sanitize(model)
	safeSerial := Sanitize(serial)
	dateTime := t.Format("2006-01-02_15-04-05")

	return fmt.Sprintf("%s - %s - %s.pdf", safeModel, safeSerial, dateTime)
}
