#!/bin/bash
set -euo pipefail
umask 077

if [[ $# -ne 1 || -e $1 ]]; then
    echo "Usage: bash $0 <new-output-directory>" >&2
    exit 1
fi

output=$1
mkdir -p "$output"
output=$(cd "$output" && pwd)
report="$output/report.txt"
failed=0
deadline=$((SECONDS + 480))

cat > "$report" <<EOF
Integration cleanup diagnostics
Collected at: $(date -u +%FT%TZ)
Workflow: ${GITHUB_WORKFLOW:-local}
Run: ${GITHUB_RUN_ID:-local}, attempt: ${GITHUB_RUN_ATTEMPT:-local}
Job: ${GITHUB_JOB:-local}
Commit: ${GITHUB_SHA:-unknown}

Resources listed here are a post-test snapshot, not necessarily leaks.
Controller models and cross-controller test fixtures may be intentional.
Each command has a stdout file and a .stderr file; exit codes appear below.
Logs may contain sensitive workload data. Review before sharing.

EOF

capture() {
    local file=$1
    shift
    local remaining=$((deadline - SECONDS))
    local status=0
    mkdir -p "$(dirname "$output/$file")"
    if ((remaining <= 0)); then
        printf 'Collection time budget exhausted\n' > "$output/$file.stderr"
        status=124
    else
        ((remaining > 20)) && remaining=20
        timeout --kill-after=2s "${remaining}s" "$@" \
            > "$output/$file" 2> "$output/$file.stderr" || status=$?
    fi
    printf '%s: exit %s\n' "$file" "$status" >> "$report"
    if ((status != 0)); then
        printf 'Failed to collect %s (exit %s); see %s.stderr\n' "$file" "$status" "$file" >&2
        failed=1
    fi
    return "$status"
}

# Cloud inventories remain useful when a model or controller has already vanished.
if command -v lxc > /dev/null; then
    capture lxd/instances.json sudo -n lxc list --format=json || :
    capture lxd/networks.json sudo -n lxc network list --format=json || :
    if capture lxd/storage.json sudo -n lxc storage list --format=json &&
        capture lxd/storage-pools.txt jq -r '.[].name' "$output/lxd/storage.json"; then
        pool_index=0
        while IFS= read -r pool; do
            pool_index=$((pool_index + 1))
            printf 'lxd/volumes-%s.json: pool %s\n' "$pool_index" "$pool" >> "$report"
            capture "lxd/volumes-$pool_index.json" sudo -n lxc storage volume list "$pool" --format=json || :
        done < "$output/lxd/storage-pools.txt"
    fi
fi
if command -v microk8s > /dev/null; then
    capture microk8s/resources.txt sudo -n microk8s kubectl get \
        namespaces,pods,services,deployments,statefulsets,jobs,pvc,pv --all-namespaces -o wide || :
    capture microk8s/events.txt sudo -n microk8s kubectl get events \
        --all-namespaces --sort-by=.metadata.creationTimestamp || :
fi

if capture controllers.json juju controllers --format=json &&
    capture controllers.txt jq -r '.controllers | keys[]' "$output/controllers.json"; then
    controller_index=0
    while IFS= read -r controller; do
        controller_index=$((controller_index + 1))
        directory="controller-$controller_index"
        printf '\n%s: %s\n' "$directory" "$controller" >> "$report"
        if ! capture "$directory/models.json" juju models --controller "$controller" --format=json; then
            continue
        fi
        if ! capture "$directory/models.tsv" jq -r \
            '.models[] |
                if (."model-uuid" | type) == "string" and (.name | type) == "string" then
                    [."model-uuid", .name] | @tsv
                else error("Model is missing a UUID or name") end' \
            "$output/$directory/models.json"; then
            continue
        fi
        model_index=0
        while IFS=$'\t' read -r uuid name; do
            model_index=$((model_index + 1))
            model_directory="$directory/model-$model_index"
            printf '%s: %s (UUID %s)\n' "$model_directory" "$name" "$uuid" >> "$report"
            model="$controller:$uuid"
            capture "$model_directory/status.json" juju status --model "$model" --format=json || :
            capture "$model_directory/status.txt" juju status --model "$model" --relations --storage || :
            capture "$model_directory/debug-log.txt" juju debug-log --model "$model" --no-tail --limit=2000 || :
        done < "$output/$directory/models.tsv"
    done < "$output/controllers.txt"
fi

printf '\nCollection finished; incomplete=%s\n' "$failed" >> "$report"
echo "Diagnostics written to $output"
exit "$failed"
