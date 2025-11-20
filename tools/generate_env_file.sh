#!/bin/bash
set -e

# generate_env_file.sh
# This script extracts the current Juju controller's environment variables
# and writes them to a test.env file. It retrieves the controller's API endpoints,
# username, password, CA certificate, cloud name, cloud type, and user system
# architecture, and writes them in KEY="value" format.
#
# This enables the developer to quickly set up environment variables for
# testing the terraform provider without needing to manually configure them.
# If the --offering-controller=<name> flag is provided,
# it also includes credentials for the specified external Juju controller.

OFFERING_CONTROLLER=""
for arg in "$@"; do
    case $arg in
        --offering-controller=*)
            OFFERING_CONTROLLER="${arg#*=}"
            shift
            ;;
        *)
            ;;
    esac
done

ENV_FILE="./test.env"

# Get current controller information
CONTROLLER=$(juju whoami | yq -r '.Controller')

# Get controller environment variables directly
JUJU_CONTROLLER_ADDRESSES=$(juju show-controller "${CONTROLLER}" | yq -r ".${CONTROLLER}.details.api-endpoints | join(\",\")")
JUJU_USERNAME=$(cat ~/.local/share/juju/accounts.yaml | yq -r ".controllers.${CONTROLLER}.user")
JUJU_PASSWORD=$(cat ~/.local/share/juju/accounts.yaml | yq -r ".controllers.${CONTROLLER}.password")
JUJU_CA_CERT=$(juju show-controller "${CONTROLLER}" | yq -r ".${CONTROLLER}.details.\"ca-cert\"" | awk '{printf "%s\\n", $0}' | sed 's/\\n$//')

# Get cloud information
CLOUD_NAME=$(juju show-controller "${CONTROLLER}" | yq -r ".${CONTROLLER}.details.cloud")
CLOUD_TYPE=$(juju show-cloud "$CLOUD_NAME" --format json | jq -r --arg cloud "$CLOUD_NAME" '.[] | select(.name == $cloud and .defined == "public") | .type')

# Map kubernetes/k8s to microk8s for testing
if [ "$CLOUD_TYPE" = "kubernetes" ] || [ "$CLOUD_TYPE" = "k8s" ]; then
    CLOUD_TYPE="microk8s"
fi


# Write environment variables to test.env file
cat > "$ENV_FILE" << EOF
TF_ACC="1"
JUJU_CONTROLLER_ADDRESSES="$JUJU_CONTROLLER_ADDRESSES"
CONTROLLER="$CONTROLLER"
JUJU_USERNAME="$JUJU_USERNAME"
JUJU_PASSWORD="$JUJU_PASSWORD"
JUJU_CA_CERT="$JUJU_CA_CERT"
TEST_CLOUD="$CLOUD_TYPE"
EOF

if [ -n "$OFFERING_CONTROLLER" ]; then
    # Get offering controller data
    OFFERING_CONTROLLER_ADDRESSES=$(juju show-controller "${OFFERING_CONTROLLER}" | yq -r ".${OFFERING_CONTROLLER}.details.api-endpoints | join(\",\")")
    OFFERING_CONTROLLER_USERAME=$(cat ~/.local/share/juju/accounts.yaml | yq -r ".controllers.${OFFERING_CONTROLLER}.user")
    OFFERING_CONTROLLER_PASSWORD=$(cat ~/.local/share/juju/accounts.yaml | yq -r ".controllers.${OFFERING_CONTROLLER}.password")
    OFFERING_CONTROLLER_CA_CERT=$(juju show-controller "${OFFERING_CONTROLLER}" | yq -r ".${OFFERING_CONTROLLER}.details.\"ca-cert\"" | awk '{printf "%s\\n", $0}' | sed 's/\\n$//')
    
    cat >> "$ENV_FILE" << EOF
OFFERING_CONTROLLER_NAME="$OFFERING_CONTROLLER"
OFFERING_CONTROLLER_ADDRESSES="$OFFERING_CONTROLLER_ADDRESSES"
OFFERING_CONTROLLER_USERNAME="$OFFERING_CONTROLLER_USERAME"
OFFERING_CONTROLLER_PASSWORD="$OFFERING_CONTROLLER_PASSWORD"
OFFERING_CONTROLLER_CA_CERT="$OFFERING_CONTROLLER_CA_CERT"
CROSS_CONTROLLERS_TESTS="1"
EOF
fi

echo "Environment variables written to $ENV_FILE"
