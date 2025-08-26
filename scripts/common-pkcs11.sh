#!/usr/bin/env bash
# Common PKCS#11 functionality for GitLab CI Runner signing
# This script should be sourced by platform-specific signing scripts

set -euo pipefail

# Global configuration - can be overridden by sourcing script
KEY_RING="${KEY_RING:-gitlab-ci-runners-signing}"
KEY_NAME="${KEY_NAME:-gitlab-runner-ssl-com}"
KEY_LOCATION="${KEY_LOCATION:-us-east1}"

GOOGLE_CLOUD_PKCS11_PROVIDER="${GOOGLE_CLOUD_PKCS11_PROVIDER:-/usr/local/lib/libkmsp11.so}"

# Global temp directory for artifacts
TEMP_DIR=$(mktemp -d)

# Ensure temp directory gets cleaned up when script exits
trap 'rm -rf "$TEMP_DIR"' EXIT

# Function to validate required environment variables
validate_environment() {
        local missing_vars=()

        if [ -z "${GCLOUD_PROJECT:-}" ]; then
                missing_vars+=("GCLOUD_PROJECT")
        fi

        if [ -z "${CERT_PATH:-}" ]; then
                missing_vars+=("CERT_PATH")
        fi

        if [ ${#missing_vars[@]} -gt 0 ]; then
                echo "Error: Missing required environment variables:"
                printf "  %s\n" "${missing_vars[@]}"
                return 1
        fi
}

# Function to check if required files exist
check_required_files() {
        # Check P11_ENGINE only if set by the calling script
        if [ -n "${P11_ENGINE:-}" ] && [ ! -f "$P11_ENGINE" ]; then
                echo "Error: PKCS#11 engine not found at $P11_ENGINE"
                if [ -n "${P11_INSTALL_INSTRUCTIONS:-}" ]; then
                        echo "$P11_INSTALL_INSTRUCTIONS"
                fi
                return 1
        fi

        if [ ! -f "$GOOGLE_CLOUD_PKCS11_PROVIDER" ]; then
                echo "Error: Google Cloud PKCS#11 provider not found at $GOOGLE_CLOUD_PKCS11_PROVIDER"
                echo "Please install it according to: https://cloud.google.com/kms/docs/reference/pkcs11-openssl"
                echo "Or set GOOGLE_CLOUD_PKCS11_PROVIDER environment variable to its location"
                return 1
        fi

        if [ ! -f "$CERT_PATH" ]; then
                echo "Error: Certificate file not found at $CERT_PATH"
                echo "Please set the GITLAB_SIGNING_CERT_PATH environment variable to your certificate location"
                return 1
        fi
}

# Function to set up the PKCS#11 environment
setup_pkcs11_environment() {
        echo "Setting up PKCS#11 signing environment..."

        # Validate environment
        validate_environment || return 1

        # Check for required files
        check_required_files || return 1

        # Create a YAML configuration file for the Google Cloud KMS PKCS#11 provider
        local kms_p11_config_file="$TEMP_DIR/kms_pkcs11.yaml"
        cat >"$kms_p11_config_file" <<EOF
---
tokens:
  - key_ring: "projects/$GCLOUD_PROJECT/locations/$KEY_LOCATION/keyRings/$KEY_RING"
EOF

        export KMS_PKCS11_CONFIG="$kms_p11_config_file"
        echo "PKCS#11 environment setup completed."
}

# Function to get the PKCS#11 key URI
get_pkcs11_key_uri() {
        echo "pkcs11:object=$KEY_NAME"
}

# Function to validate input file
validate_input_file() {
        local input_file="$1"

        if [ -z "$input_file" ]; then
                echo "Error: Input filename is required"
                return 1
        fi

        if [ ! -f "$input_file" ]; then
                echo "Error: Input file '$input_file' not found"
                return 1
        fi
}

# Function to prepare output file path
prepare_output_path() {
        local input_file="$1"
        local output_file="${2:-$input_file}"

        # Check if we're overwriting the input file
        if [ "$input_file" = "$output_file" ]; then
                echo "$TEMP_DIR/$(basename "$input_file").signed"
        else
                mkdir -p "$(dirname "$output_file")"
                echo "$output_file"
        fi
}

# Function to handle file replacement after signing
finalize_signed_file() {
        local input_file="$1"
        local temp_output="$2"
        local final_output="$3"

        if [ "$input_file" = "$final_output" ]; then
                echo "Creating backup: $input_file.unsigned"
                mv "$input_file" "$input_file.unsigned"
                mv "$temp_output" "$final_output"
                echo "Signed file: $final_output"
        else
                echo "Signed file: $temp_output"
        fi
}
