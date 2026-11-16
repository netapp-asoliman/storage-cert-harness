#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=../partner-harness-common.sh
source "${repo_root}/ci/scripts/partner-harness-common.sh"

tmp="$(mktemp -d)"
trap 'rm -rf -- "${tmp}"' EXIT
printf 'kubeconfig\n' > "${tmp}/kubeconfig"
podman() {
	printf '%s\n' "$@" >> "${tmp}/podman-args"
	if [[ "$1 $2" == "image exists" && "$3" != "local-image" ]]; then
		return 1
	fi
}

unset USE_LOCAL_IMAGE HARNESS_IMAGE
partner_prepare_image remote-image
grep -Fxq 'pull' "${tmp}/podman-args"
grep -Fxq 'remote-image' "${tmp}/podman-args"

REPO_ROOT="${repo_root}"
: > "${tmp}/podman-args"
partner_run_harness_container remote-image "${tmp}/kubeconfig" \
	"${repo_root}/backends.example.yaml" example-array preflight
! grep -Fxq -- '--pull=never' "${tmp}/podman-args"

: > "${tmp}/podman-args"
USE_LOCAL_IMAGE=true
HARNESS_IMAGE=local-image
partner_prepare_image "${HARNESS_IMAGE}"
grep -Fxq 'exists' "${tmp}/podman-args"
! grep -Fxq 'pull' "${tmp}/podman-args"

: > "${tmp}/podman-args"
partner_run_harness_container "${HARNESS_IMAGE}" "${tmp}/kubeconfig" \
	"${repo_root}/backends.example.yaml" example-array preflight
grep -Fxq -- '--pull=never' "${tmp}/podman-args"
grep -Fxq 'local-image' "${tmp}/podman-args"

HARNESS_IMAGE=missing-image
if partner_prepare_image "${HARNESS_IMAGE}" >"${tmp}/error" 2>&1; then
	echo 'expected missing local image to fail' >&2
	exit 1
fi
grep -Fq 'local image missing-image is unavailable' "${tmp}/error"
unset HARNESS_IMAGE
if partner_prepare_image remote-image >"${tmp}/error" 2>&1; then
	echo 'expected missing HARNESS_IMAGE to fail' >&2
	exit 1
fi
grep -Fq 'requires HARNESS_IMAGE' "${tmp}/error"
USE_LOCAL_IMAGE=invalid
if partner_prepare_image remote-image >"${tmp}/error" 2>&1; then
	echo 'expected invalid USE_LOCAL_IMAGE to fail' >&2
	exit 1
fi
grep -Fq 'must be true or false' "${tmp}/error"
echo 'partner image checks passed'
