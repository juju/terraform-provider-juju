#!/bin/bash
set -euo pipefail

# The test script doubles as the mocked CLI through small wrapper commands.
case ${MOCK_COMMAND:-} in
    juju)
        printf '%s\n' "$*" >> "$MOCK_CALLS"
        case $1 in
            version) echo '4.0.0-test' ;;
            controllers)
                case $SCENARIO in
                    malformed) echo 'not json' ;;
                    empty) echo '{"controllers":{}}' ;;
                    *) echo '{"controllers":{"main":{},"offering":{}}}' ;;
                esac
                ;;
            show-controller)
                if [[ $SCENARIO == no-nodes ]]; then
                    printf '{"%s":{"details":{}}}\n' "$2"
                elif [[ $2 == main ]]; then
                    echo '{"main":{"controller-machines":{"0":{},"1":{}}}}'
                else
                    echo '{"offering":{"controller-nodes":{"0":{}}}}'
                fi
                ;;
            ssh)
                [[ $2 == --model && $4 == --pty=false && $6 == bash && $7 == -lc ]]
                case $8 in
                    juju_engine_report|juju_goroutines|juju_metrics) ;;
                    *) echo "Unexpected introspection command: $8" >&2; exit 1 ;;
                esac
                if [[ $SCENARIO == failed-introspection && $3 == main:controller && $8 == juju_engine_report ]]; then
                    echo 'Introspection socket unavailable' >&2
                    exit 1
                fi
                printf 'Introspection %s on %s node %s\n' "$8" "$3" "$5"
                ;;
            models)
                if [[ $SCENARIO == unreachable && $3 == main ]]; then
                    echo 'Controller unavailable' >&2
                    /bin/sleep 30
                fi
                if [[ $SCENARIO == empty-models ]]; then
                    echo '{"models":[]}'
                elif [[ $SCENARIO == invalid-model && $3 == main ]]; then
                    echo '{"models":[{"name":"missing-uuid"}]}'
                elif [[ $3 == main ]]; then
                    echo '{"models":[{"model-uuid":"uuid-1","name":"admin/controller"},{"model-uuid":"uuid-2","name":"other-owner/dying-model","life":"dying"}]}'
                else
                    echo '{"models":[{"model-uuid":"uuid-3","name":"admin/offering-model"}]}'
                fi
                ;;
            status)
                if [[ $SCENARIO == failed-status && $3 == main:uuid-2 ]]; then
                    echo 'Model is dying' >&2
                    exit 1
                fi
                echo '{"applications":{"leftover":{"life":"dying"}}}'
                ;;
            debug-log)
                [[ $4 == --no-tail && $5 == --limit=2000 ]]
                echo 'Recent model log'
                ;;
            *)
                echo "Unexpected Juju command: $*" >&2
                exit 1
                ;;
        esac
        exit 0
        ;;
    jq)
        # Snap-confined tools can read stdin but cannot open the report's paths.
        for argument in "$@"; do
            if [[ -f $argument ]]; then
                echo "Permission denied: jq cannot open $argument" >&2
                exit 2
            fi
        done
        exec "$REAL_JQ" "$@"
        ;;
    sudo)
        [[ $1 == -n ]]
        shift
        exec "$@"
        ;;
    lxc|microk8s)
        printf '%s %s\n' "$MOCK_COMMAND" "$*" >> "$MOCK_CALLS"
        if [[ $MOCK_COMMAND == lxc && $1 == storage && $2 == list ]]; then
            echo '[{"name":"default"}]'
        else
            echo 'Cloud inventory'
        fi
        exit 0
        ;;
esac

tools=$(cd "$(dirname "$0")" && pwd)
workspace=$(mktemp -d)
trap 'rm -r -- "$workspace"' EXIT
export MOCK_CALLS="$workspace/calls"
export REAL_JQ
REAL_JQ=$(command -v jq)
export GITHUB_WORKSPACE="$workspace/test-workspace"
export GITHUB_OUTPUT="$workspace/github-output"
export GITHUB_SERVER_URL=https://github.com GITHUB_REPOSITORY=juju/terraform-provider-juju
export GITHUB_RUN_ID=123456 GITHUB_RUN_NUMBER=42 GITHUB_RUN_ATTEMPT=2
export DIAGNOSTICS_VARIANT=lxd-juju4.0/edge-1
export DIAGNOSTICS_MATRIX='{"cloud":"lxd","juju":"4.0/edge","terraform":"1.14.*"}'
mkdir "$GITHUB_WORKSPACE"
echo '{"Test":"cleanup","Action":"fail"}' > "$GITHUB_WORKSPACE/integration-test.json"
echo 'Test cleanup failed' > "$GITHUB_WORKSPACE/integration-test.log"
mkdir "$workspace/bin"
for utility in cat mkdir dirname date timeout; do
    ln -s "$(command -v "$utility")" "$workspace/bin/$utility"
done
# Use Bash for mock entrypoints so this test does not require executable source files.
for utility in juju sudo lxc microk8s jq; do
    printf '#!/bin/bash\nexport MOCK_COMMAND=%s\nexec /bin/bash "%s" "$@"\n' \
        "$utility" "$tools/collect_integration_diagnostics_test.sh" > "$workspace/bin/$utility"
    chmod +x "$workspace/bin/$utility"
done

run_case() {
    export SCENARIO=$1
    local expected=$2
    local status=0
    : > "$MOCK_CALLS"
    PATH="$workspace/bin" /bin/bash "$tools/collect_integration_diagnostics.sh" \
        "$workspace/$SCENARIO" > "$workspace/stdout" 2> "$workspace/stderr" || status=$?
    if [[ $status != "$expected" ]]; then
        cat "$workspace/stderr" >&2
        echo "FAIL: $SCENARIO exited $status, expected $expected" >&2
        exit 1
    fi
}

run_case success 0
grep -qx 'artifact-name=juju-diag-lxd-juju4.0-edge-1-r42-a2' "$GITHUB_OUTPUT"
grep -q 'Run URL: https://github.com/juju/terraform-provider-juju/actions/runs/123456' "$workspace/success/report.txt"
grep -Fq "$DIAGNOSTICS_MATRIX" "$workspace/success/report.txt"
cmp "$GITHUB_WORKSPACE/integration-test.json" "$workspace/success/integration-test.json"
cmp "$GITHUB_WORKSPACE/integration-test.log" "$workspace/success/integration-test.log"
grep -q '4.0.0-test' "$workspace/success/juju-version.txt"
grep -q 'Introspection juju_engine_report on main:controller node 0' "$workspace/success/controller-1/agent-1/engine-report.txt"
grep -q 'Introspection juju_goroutines on main:controller node 1' "$workspace/success/controller-1/agent-2/goroutines.txt"
grep -q 'Introspection juju_metrics on offering:controller node 0' "$workspace/success/controller-2/agent-1/metrics.txt"
grep -q 'other-owner/dying-model (UUID uuid-2)' "$workspace/success/report.txt"
grep -q 'Recent model log' "$workspace/success/controller-2/model-1/debug-log.txt"
grep -q -- 'status --model main:uuid-1 --format=json' "$MOCK_CALLS"
grep -q 'Cloud inventory' "$workspace/success/lxd/instances.json"
grep -q 'Cloud inventory' "$workspace/success/lxd/volumes-1.json"
grep -q 'Cloud inventory' "$workspace/success/microk8s/events.txt"
if grep -Eq 'switch|show-credentials|show-secrets|show-password|dump-model|^exec ' "$MOCK_CALLS"; then
    echo 'FAIL: collector used a state-changing or secret-exporting command' >&2
    exit 1
fi

run_case failed-introspection 1
grep -q 'Introspection socket unavailable' "$workspace/failed-introspection/controller-1/agent-1/engine-report.txt.stderr"
grep -q 'Introspection juju_goroutines' "$workspace/failed-introspection/controller-1/agent-1/goroutines.txt"
grep -q 'Recent model log' "$workspace/failed-introspection/controller-2/model-1/debug-log.txt"

run_case no-nodes 0
grep -q 'No controller nodes exposed' "$workspace/no-nodes/report.txt"
test ! -d "$workspace/no-nodes/controller-1/agent-1"
grep -q 'Recent model log' "$workspace/no-nodes/controller-1/model-1/debug-log.txt"

run_case failed-status 1
grep -q 'Model is dying' "$workspace/failed-status/controller-1/model-2/status.json.stderr"
grep -q 'status.json: exit 1' "$workspace/failed-status/report.txt"
grep -q 'Recent model log' "$workspace/failed-status/controller-1/model-2/debug-log.txt"
grep -q 'Recent model log' "$workspace/failed-status/controller-2/model-1/debug-log.txt"

started=$SECONDS
run_case unreachable 1
elapsed=$((SECONDS - started))
if ((elapsed < 20 || elapsed > 25)); then
    echo "FAIL: command timeout took $elapsed seconds, expected 20-25" >&2
    exit 1
fi
grep -q 'models.json: exit 124' "$workspace/unreachable/report.txt"
grep -q 'Recent model log' "$workspace/unreachable/controller-2/model-1/debug-log.txt"

run_case malformed 1
test -s "$workspace/malformed/controllers.txt.stderr"
grep -q 'Cloud inventory' "$workspace/malformed/lxd/instances.json"

run_case invalid-model 1
test -s "$workspace/invalid-model/controller-1/models.tsv.stderr"
grep -q 'Recent model log' "$workspace/invalid-model/controller-2/model-1/debug-log.txt"

run_case empty 0
test ! -d "$workspace/empty/controller-1"

run_case empty-models 0
test ! -d "$workspace/empty-models/controller-1/model-1"
test ! -d "$workspace/empty-models/controller-2/model-1"

rm "$workspace/bin/lxc" "$workspace/bin/microk8s" "$workspace/bin/sudo"
run_case no-cloud-tools 0
test ! -d "$workspace/no-cloud-tools/lxd"
test ! -d "$workspace/no-cloud-tools/microk8s"

rm "$workspace/bin/jq"
run_case missing-jq 1
grep -q 'controllers.txt: exit 127' "$workspace/missing-jq/report.txt"

rm "$workspace/bin/juju"
run_case missing-juju 1
grep -q 'controllers.json: exit 127' "$workspace/missing-juju/report.txt"

if /bin/bash "$tools/collect_integration_diagnostics.sh" "$workspace/success" \
    > "$workspace/stdout" 2> "$workspace/stderr"; then
    echo 'FAIL: collector accepted a pre-existing output directory' >&2
    exit 1
fi
echo 'All integration diagnostics collector tests passed.'
