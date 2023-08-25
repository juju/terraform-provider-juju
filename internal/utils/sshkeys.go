// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package utils

import "strings"

// GetKeyIdentifierFromSSHKey returns the identifier of the key,
// which is currently based on the comment field (TODO issue #267)
func GetKeyIdentifierFromSSHKey(key string) string {
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
