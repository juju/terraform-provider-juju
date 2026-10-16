#!/bin/bash
set -euo pipefail

# The test script doubles as the mocked CLI through small wrapper commands.
case ${MOCK_COMMAND:-} in
    juju)
        printf '%s\n' "$*" >> "$MOCK_CALLS"
        case $1 in
            controllers)
                case $SCENARIO in
                    malformed) echo 'not json' ;;
                    empty) echo '{"controllers":{}}' ;;
                    *) echo '{"controllers":{"main":{},"offering":{}}}' ;;
                esac
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
mkdir "$workspace/bin"
for utility in cat mkdir dirname date jq timeout; do
    ln -s "$(command -v "$utility")" "$workspace/bin/$utility"
done
# Use Bash for mock entrypoints so this test does not require executable source files.
for utility in juju sudo lxc microk8s; do
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
grep -q 'other-owner/dying-model (UUID uuid-2)' "$workspace/success/report.txt"
grep -q 'Recent model log' "$workspace/success/controller-2/model-1/debug-log.txt"
grep -q -- 'status --model main:uuid-1 --format=json' "$MOCK_CALLS"
grep -q 'Cloud inventory' "$workspace/success/lxd/instances.json"
grep -q 'Cloud inventory' "$workspace/success/lxd/volumes-1.json"
grep -q 'Cloud inventory' "$workspace/success/microk8s/events.txt"
if grep -Eq 'switch|show-credentials|show-secrets|dump-model' "$MOCK_CALLS"; then
    echo 'FAIL: collector used a state-changing or secret-exporting command' >&2
    exit 1
fi

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
