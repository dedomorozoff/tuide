package sys

import "os"

func homeDir() string {
	return os.Getenv("USERPROFILE")
}

func configBase() string {
	return firstNonEmpty(os.Getenv("APPDATA"), homeDir())
}

func dataBase() string {
	return firstNonEmpty(os.Getenv("LOCALAPPDATA"), os.Getenv("APPDATA"), homeDir())
}

func cacheBase() string {
	return firstNonEmpty(os.Getenv("LOCALAPPDATA"), os.Getenv("TEMP"), homeDir())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
