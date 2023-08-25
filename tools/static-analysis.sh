#!/bin/bash

supports_colors() {
	if [[ -z ${TERM} ]] || [[ ${TERM} == "" ]] || [[ ${TERM} == "dumb" ]]; then
		echo "NO"
		return
	fi
	if which tput >/dev/null 2>&1; then
		# shellcheck disable=SC2046
		if [[ $(tput colors) -gt 1 ]]; then
			echo "YES"
			return
		fi
	fi
	echo "NO"
}

red() {
	if [[ "$(supports_colors)" == "YES" ]]; then
		tput sgr0
		echo "$(tput setaf 1)${1}$(tput sgr0)"
		return
	fi
	echo "${1}"
}

green() {
	if [[ "$(supports_colors)" == "YES" ]]; then
		tput sgr0
		echo "$(tput setaf 2)${1}$(tput sgr0)"
		return
	fi
	echo "${1}"
}


OUTPUT() {
	local output

	output=${1}
	shift

	if [[ -z ${output} ]] || [[ ${VERBOSE} -gt 1 ]]; then
		echo
	fi

	# shellcheck disable=SC2162
	while read data; do
		# If there is no output, just dump straight to stdout.
		if [[ -z ${output} ]]; then
			echo "${data}"
		# If there is an output, but we're not in verbose mode, just append to
		# the output.
		elif [[ ${VERBOSE} -le 1 ]]; then
			echo "${data}" >>"${output}"
		# If we are in verbose mode, but we're an empty line, send to stdout
		# and also tee it to the output.
		elif echo "${data}" | grep -q "^\s*$"; then
			echo "${data}" | tee -a "${output}"
		# Finally, we have content and we're in verbose mode. Send the data to
		# the output and then format it for stdout.
		else
			echo "${data}" | tee -a "${output}" | sed 's/^/    | /g'
		fi
	done

	if [[ -z ${output} ]] || [[ ${VERBOSE} -gt 1 ]]; then
		echo
	fi
}

# run_linter will run until the end of a pipeline even if there is a failure.
# This is different from `run` as we require the output of a linter.
run_linter() {
	CMD="${1}"

	if [[ -n ${RUN_SUBTEST} ]]; then
		# shellcheck disable=SC2143
		if [[ ! "$(echo "${RUN_SUBTEST}" | grep -E "^${CMD}$")" ]]; then
			echo "SKIPPING: ${RUN_SUBTEST} ${CMD}"
			exit 0
		fi
	fi

	DESC=$(echo "${1}" | sed -E "s/^run_//g" | sed -E "s/_/ /g")

	echo "===> [   ] Running: ${DESC}"

	START_TIME=$(date +%s)

	# Prevent the sub-shell from killing our script if that sub-shell fails on an
	# error. We need this so that we can capture the full output and collect the
	# exit code when it does fail.
	# Do not remove or none of the tests will report correctly!
	set +e
	set -o pipefail

	cmd_output=$("${CMD}" "$@" 2>&1)
	cmd_status=$?

	set +o pipefail

	# Only output if it's not empty.
	if [[ -n ${cmd_output} ]]; then
		echo -e "${cmd_output}" | OUTPUT
	fi

	END_TIME=$(date +%s)

	if [[ ${cmd_status} -eq 0 ]]; then
		echo -e "\r\033[1A\033[0K===> [ $(green "✔") ] Success: ${DESC} ($((END_TIME - START_TIME))s)"
	else
		echo -e "\r\033[1A\033[0K===> [ $(red "x") ] Fail: ${DESC} ($((END_TIME - START_TIME))s)"
		exit 1
	fi
}


run_go() {
	VER=$(golangci-lint --version | tr -s ' ' | cut -d ' ' -f 4 | cut -d '.' -f 1,2)
	if [[ ${VER} != "1.53" ]] && [[ ${VER} != "v1.53" ]]; then
		(echo >&2 -e '\nError: golangci-lint version does not match 1.53. Please upgrade/downgrade to the right version.')
		exit 1
	fi
	OUT=$(golangci-lint run -c .github/golangci-lint.config.yaml 2>&1)
	if [[ -n ${OUT} ]]; then
		(echo >&2 "\\nError: linter has issues:\\n\\n${OUT}")
		exit 1
	fi
	OUT=$(golangci-lint run -c .github/golangci-lint.config.experimental.yaml 2>&1)
	if [[ -n ${OUT} ]]; then
		(echo >&2 "\\nError: experimental linter has issues:\\n\\n${OUT}")
		exit 1
	fi
}

run_go_tidy() {
	CUR_SHA=$(git show HEAD:go.sum | shasum -a 1 | awk '{ print $1 }')
	go mod tidy 2>&1
	NEW_SHA=$(cat go.sum | shasum -a 1 | awk '{ print $1 }')
	if [[ ${CUR_SHA} != "${NEW_SHA}" ]]; then
		git diff >&2
		(echo >&2 -e "\\nError: go mod sum is out of sync. Run 'go mod tidy' and commit source.")
		exit 1
	fi
}

run_copyright() {
	OUT=$(find . -name '*.go' | sort | xargs grep -L -E '// (Copyright|Code generated)' || true)
	LINES=$(echo "${OUT}" | wc -w)
	if [ "$LINES" != 0 ]; then
		echo ""
		echo "$(red 'Found some issues:')"
		echo -e '\nThe following files are missing copyright headers'
		echo "${OUT}"
		exit 1
	fi
}

(
	run_linter "run_go"
	run_linter "run_go_tidy"
	run_linter "run_copyright"
)
