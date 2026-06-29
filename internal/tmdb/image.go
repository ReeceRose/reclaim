package tmdb

import "fmt"

func PosterURL(path, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s%s", imageBaseURL, size, path)
}

func BackdropURL(path, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s%s", imageBaseURL, size, path)
}
