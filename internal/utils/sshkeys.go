package utils

import "strings"

// GetUserFromSSHKey returns the user of the key
// returning the string after the "=" character
func GetUserFromSSHKey(key string) string {
	end := strings.LastIndex(key, "=")
	if end < 0 {
		return ""
	}
	if (end + 2) >= len(key) {
		return ""
	}
	user := key[end+2:]
	return user
}
