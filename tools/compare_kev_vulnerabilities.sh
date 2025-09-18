#!/bin/bash
#
# Copyright 2025 Canonical Ltd. All rights reserved.

set -euo pipefail

# This script compares known vulnerabilities from the CISA KEV list
# with the vulnerabilities found in a vulnerability report file.
# KEV = Known Exploited Vulnerabilities
#
# The KEV file is provided in JSON format from CISA at
# https://www.cisa.gov/known-exploited-vulnerabilities-catalog
#
# The vulnerability report file should be in SARIF format, as this
# provides interoperability across different scanners.
# SARIF is just JSON with a specific schema.

# USAGE: VULN_REPORT_FILE=my-trivy.sarif KNOWN_CVES_FILE=my-known-vulns.sarif ./compare_kev_vulnerabilities.sh

# Always run from repo root, assuming we are in scripts/
cd "$(dirname "$0")/.."

KNOWN_CVES_FILE="${KNOWN_CVES_FILE:-kev.json}"

# Ensure files exist
if [ ! -f "$VULN_REPORT_FILE" ]; then
  echo "Error: Vulnerability report file '$VULN_REPORT_FILE' not found."
  exit 2
fi

if [ ! -f "$KNOWN_CVES_FILE" ]; then
  echo "Error: Known CVEs file '$KNOWN_CVES_FILE' not found."
  exit 2
fi

# Extract CVE IDs from known vulnerabilities list
known_cves="$(jq -r '.vulnerabilities[].cveID' "$KNOWN_CVES_FILE" | sort -u)"
report_cves="$(jq -r '.runs[].results[].ruleId' "$VULN_REPORT_FILE" | sort -u)"

# Find matches
matches="$(echo "$report_cves" | grep -F -f <(echo "$known_cves") || true)"

if [ -n "$matches" ]; then
  echo "Known vulnerabilities found:"
  echo "$matches"
  exit 1
fi

echo "No known vulnerabilities found."
