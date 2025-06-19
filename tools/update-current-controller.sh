#!/bin/bash
set -e

# update-current-controller.sh
# This script updates the ./vscode/settings.json file with the current Juju controller's
# environment variables. It retrieves the controller's API endpoints, username,
# password, CA certificate, cloud name, cloud type, and user system architecture,
# and writes them into the settings.json file under the "go.testEnvVars" key.
#
# This enables the developer to quickly switch controllers when working with
# the terraform providers acceptance tests, without needing to manually
# update the settings file.
#
# Any existing env vars you've set will be untouched and only the named ones updated.

SETTINGS_FILE="./.vscode/settings.json"
if [ ! -f "$SETTINGS_FILE" ]; then
  echo '{"go.testEnvVars":{}}' > "$SETTINGS_FILE"
fi

CONTROLLER=$(juju whoami | yq -r '.Controller')
JUJU_CONTROLLER_ADDRESSES=$(juju show-controller | yq -r ".${CONTROLLER}.details.api-endpoints | join(\",\")")
JUJU_USERNAME=$(cat ~/.local/share/juju/accounts.yaml | yq -r ".controllers.${CONTROLLER}.user")
JUJU_PASSWORD=$(cat ~/.local/share/juju/accounts.yaml | yq -r ".controllers.${CONTROLLER}.password")
JUJU_CA_CERT=$(juju show-controller "${CONTROLLER}" | yq -r ".${CONTROLLER}.details.\"ca-cert\"" | sed 's/\\n/\n/g')

CLOUD_NAME=$(juju show-controller a | yq '.a.details.cloud')
CLOUD_TYPE=$(juju show-cloud $CLOUD_NAME --format json | jq -r --arg cloud "$CLOUD_NAME" '.[] | select(.name == $cloud and .defined == "public") | .type')

TMP_FILE="${SETTINGS_FILE}.tmp"

SYSTEM_ARCHITECTURE=$(uname -m)

jq --arg ca_cert "$JUJU_CA_CERT" \
   --arg ctrl_addr "$JUJU_CONTROLLER_ADDRESSES" \
   --arg username "$JUJU_USERNAME" \
   --arg password "$JUJU_PASSWORD" \
   --arg controller "$CONTROLLER" \
   --arg cloudtype "$CLOUD_TYPE" \
   --arg sysarch "$SYSTEM_ARCHITECTURE" \
   '.["go.testEnvVars"]["JUJU_CA_CERT"] = $ca_cert
    | .["go.testEnvVars"]["JUJU_CONTROLLER_ADDRESSES"] = $ctrl_addr
    | .["go.testEnvVars"]["JUJU_USERNAME"] = $username
    | .["go.testEnvVars"]["JUJU_PASSWORD"] = $password
    | .["go.testEnvVars"]["CONTROLLER"] = $controller
    | .["go.testEnvVars"]["TEST_CLOUD"] = $cloudtype
    | .["go.testEnvVars"]["JUJU_DEFAULT_TEST_MODEL_ARCHITECTURE"] = $sysarch
   ' "$SETTINGS_FILE" > "$TMP_FILE" && mv "$TMP_FILE" "$SETTINGS_FILE"

echo "Updated Juju env vars in $SETTINGS_FILE"
