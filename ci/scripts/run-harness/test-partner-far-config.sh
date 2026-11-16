#!/usr/bin/env bash
# Cluster-free checks for FAR host-file validation and the preflight mount.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=../partner-harness-common.sh
source "${repo_root}/ci/scripts/partner-harness-common.sh"

tmp="$(mktemp -d)"
trap 'rm -rf -- "${tmp}"' EXIT
cp "${repo_root}/examples/far-config.example.json" "${tmp}/far-config.json"
printf 'kubeconfig\n' > "${tmp}/kubeconfig"
cat > "${tmp}/plan.yaml" <<'YAML'
select:
  ids:
    - TR-VIRT-007
overrides:
  TR-VIRT-007:
    mode: far-operator
    node: worker-0.example.com
    far_config: /far-config.json
YAML

node_state=ready
oc() {
	[[ "$*" == *'get node worker-0.example.com -o json'* ]] || return 1
	[[ "${node_state}" != missing ]] || return 1
	local role='"node-role.kubernetes.io/worker":""' ready=True cordoned=false
	[[ "${node_state}" != not-worker ]] || role='"node-role.kubernetes.io/control-plane":""'
	[[ "${node_state}" != not-ready ]] || ready=False
	[[ "${node_state}" != cordoned ]] || cordoned=true
	printf '{"metadata":{"labels":{%s}},"spec":{"unschedulable":%s},"status":{"conditions":[{"type":"Ready","status":"%s"}]}}\n' "${role}" "${cordoned}" "${ready}"
}

FAR_CONFIG="${tmp}/far-config.json"
partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" "${tmp}/kubeconfig"
[[ "${PARTNER_FAR_MOUNT}" == "${FAR_CONFIG}:/far-config.json:ro,Z" ]]

# Capture Podman arguments without pulling images or contacting a cluster.
podman() { printf '%s\n' "$@" > "${tmp}/podman-args"; }
REPO_ROOT="${repo_root}"
partner_run_harness_container image "${tmp}/kubeconfig" "${repo_root}/backends.example.yaml" example-array preflight --plan /plans/level3-customer-no-load-80.yaml
grep -Fxq "${PARTNER_FAR_MOUNT}" "${tmp}/podman-args"

# With no FAR execution selected, even a missing FAR_CONFIG must not be required.
sed 's/TR-VIRT-007/TR-STOR-002/g' "${tmp}/plan.yaml" > "${tmp}/no-far.yaml"
FAR_CONFIG="${tmp}/does-not-exist.json"
partner_validate_far_config "${tmp}/no-far.yaml" "${repo_root}" "${tmp}/kubeconfig"
[[ -z "${PARTNER_FAR_MOUNT}" ]]
partner_run_harness_container image "${tmp}/kubeconfig" "${repo_root}/backends.example.yaml" example-array preflight --plan /plans/level3-customer-load-80.yaml
! grep -Fq ':/far-config.json:ro,Z' "${tmp}/podman-args"

# Monitor-only mode does not need a FAR manifest.
sed 's/far-operator/monitor/' "${tmp}/plan.yaml" > "${tmp}/monitor.yaml"
partner_validate_far_config "${tmp}/monitor.yaml" "${repo_root}" "${tmp}/kubeconfig"
[[ -z "${PARTNER_FAR_MOUNT}" ]]

for bad in "${tmp}/does-not-exist.json" "${repo_root}/examples/far-config.example.json"; do
	FAR_CONFIG="${bad}"
	if partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" "${tmp}/kubeconfig" >"${tmp}/error" 2>&1; then
		echo "expected FAR_CONFIG rejection: ${bad}" >&2
		exit 1
	fi
	[[ -z "${PARTNER_FAR_MOUNT}" ]]
done

FAR_CONFIG="${tmp}/far-config.json"
sed 's@/far-config.json@/other-config.json@' "${tmp}/plan.yaml" > "${tmp}/wrong-path.yaml"
if partner_validate_far_config "${tmp}/wrong-path.yaml" "${repo_root}" "${tmp}/kubeconfig" >"${tmp}/error" 2>&1; then
	echo 'expected container-path mismatch rejection' >&2
	exit 1
fi
sed 's/worker-0.example.com/worker-1.example.com/' "${tmp}/plan.yaml" > "${tmp}/wrong-node.yaml"
if partner_validate_far_config "${tmp}/wrong-node.yaml" "${repo_root}" "${tmp}/kubeconfig" >"${tmp}/error" 2>&1; then
	echo 'expected FAR target mismatch rejection' >&2
	exit 1
fi

sed 's/"fence-agents-credentials-shared"/""/' "${tmp}/far-config.json" > "${tmp}/missing-secret.json"
FAR_CONFIG="${tmp}/missing-secret.json"
if partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" "${tmp}/kubeconfig" >"${tmp}/error" 2>&1; then
	echo 'expected missing Secret reference rejection' >&2
	exit 1
fi

sed 's/"worker-0.example.com": "192.0.2.10"/"worker-1.example.com": "192.0.2.10"/' "${tmp}/far-config.json" > "${tmp}/wrong-ip-key.json"
FAR_CONFIG="${tmp}/wrong-ip-key.json"
if partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" "${tmp}/kubeconfig" >"${tmp}/error" 2>&1; then
	echo 'expected missing target BMC address rejection' >&2
	exit 1
fi

"$(partner_resolve_yq)" e -o=json -I=2 \
	'del(.spec.nodeparameters."--ip") | del(.spec.sharedSecretName) | .spec.agent = "fence_other"' \
	"${tmp}/far-config.json" > "${tmp}/other-agent.json"
FAR_CONFIG="${tmp}/other-agent.json"
partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" "${tmp}/kubeconfig"
[[ "${PARTNER_FAR_MOUNT}" == "${FAR_CONFIG}:/far-config.json:ro,Z" ]]

FAR_CONFIG="${tmp}/far-config.json"
for node_state in missing not-ready not-worker cordoned; do
	if partner_validate_far_config "${tmp}/plan.yaml" "${repo_root}" "${tmp}/kubeconfig" >"${tmp}/error" 2>&1; then
		echo "expected ${node_state} target rejection" >&2
		exit 1
	fi
	[[ -z "${PARTNER_FAR_MOUNT}" ]]
done
echo 'partner FAR mount checks passed'
