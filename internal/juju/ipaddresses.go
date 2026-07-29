// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"
	"net/netip"

	jujuerrors "github.com/juju/errors"
)

// Special aliases supported by MatchIPAddresses.
const (
	// IPAddressConditionAny matches any IP address.
	IPAddressConditionAny = "any"
	// IPAddressConditionPublic matches any non-private IP address.
	IPAddressConditionPublic = "public"
	// IPAddressConditionPrivate matches any private IP address.
	IPAddressConditionPrivate = "private"
)

// ErrNoMatchingIPAddress is returned by MatchIPAddresses when a condition
// cannot be satisfied with the currently reported IP addresses. It is a
// retriable error: the caller should keep waiting for more IPs to be
// provisioned.
var ErrNoMatchingIPAddress = jujuerrors.ConstError("no-ip-matching")

// ValidIPAddressCondition reports whether the given condition is a valid
// wait-for-ip-addresses condition: one of the aliases or a valid CIDR.
func ValidIPAddressCondition(condition string) bool {
	switch condition {
	case IPAddressConditionAny, IPAddressConditionPublic, IPAddressConditionPrivate:
		return true
	}
	_, err := netip.ParsePrefix(condition)
	return err == nil
}

// isPrivateIP reports whether addr is a private or otherwise non-public
// address, using the standard library predicates: RFC 1918/4193 (IsPrivate),
// loopback, and link-local.
func isPrivateIP(addr netip.Addr) bool {
	return addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast()
}

func ipMatchesCondition(addr netip.Addr, condition string) bool {
	switch condition {
	case IPAddressConditionAny:
		return true
	case IPAddressConditionPublic:
		return !isPrivateIP(addr)
	case IPAddressConditionPrivate:
		return isPrivateIP(addr)
	}
	prefix, err := netip.ParsePrefix(condition)
	if err != nil {
		return false
	}
	return prefix.Contains(addr)
}

// MatchIPAddresses matches the machine's reported IP addresses against the
// list of wait conditions. It returns one IP address per condition, in the
// same order, with each condition consuming a distinct IP.
// If a condition cannot be satisfied, ErrNoMatchingIPAddress is returned.
func MatchIPAddresses(ips []string, conditions []string) ([]string, error) {
	matched := make([]string, len(conditions))
	used := make(map[string]bool)

	for i, cond := range conditions {
		// Pick the first candidate that matches the condition and hasn't been
		// used by a previous condition.
		for _, ip := range ips {
			if used[ip] {
				continue
			}
			addr, err := netip.ParseAddr(ip)
			if err != nil {
				return nil, fmt.Errorf("parsing IP address %q: %w", ip, err)
			}
			if !ipMatchesCondition(addr, cond) {
				continue
			}
			used[ip] = true
			matched[i] = ip
			break
		}

		if matched[i] == "" {
			return nil, jujuerrors.WithType(jujuerrors.Errorf("%q among reported IPs %v", cond, ips), ErrNoMatchingIPAddress)
		}
	}
	return matched, nil
}
