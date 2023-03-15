package utils

import "strings"

// GetUserFromSSHKey returns the user of the key
func GetUserFromSSHKey(key string) string {
	// The key is broken down into component values.
	// components[0] is the type of key (e.g. ssh-rsa)
	// components[1] is the key string itself
	// components[2] is the key's comment field (e.g. user@server)
	components := strings.Fields(key)
	if len(components) < 3 {
		return ""
	} else {
		return components[2]
	}
}
