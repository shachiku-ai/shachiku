package provider

import (
	"path/filepath"
	"regexp"
	"strings"

	"shachiku/core/config"
)

var imageRegex = regexp.MustCompile(`!\[.*?\]\((?:http[s]?:[/\\]+[^\s/]+)?/api/uploads/([^)]+)\)`)

func extractImagesAndText(content string) (string, []string) {
	var images []string
	cleanText := imageRegex.ReplaceAllStringFunc(content, func(m string) string {
		matches := imageRegex.FindStringSubmatch(m)
		if len(matches) > 1 {
			filename := matches[1]
			images = append(images, filepath.Join(config.GetDataDir(), "uploads", filename))
		}
		return ""
	})
	return strings.TrimSpace(cleanText), images
}
