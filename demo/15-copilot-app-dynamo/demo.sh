#!/usr/bin/env bash
# Orka — the Copilot app, with Kubernetes as the backend
# The terminal scenes of a narrated walkthrough. The Copilot app and browser clips are recorded separately; see README.md.
# Usage: demo.sh install | connect | model | result <task> <pr-number>
# shellcheck disable=SC2016
here=$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
: "${ORKA_DEMO_ENV:=$here/../../bin/copilot-dynamo/env.sh}"
export ORKA_DEMO_ENV
# shellcheck source=demo/lib/demo.sh
source "$here/../lib/demo.sh"
# The values file sits next to the env, so the typed path stays short.
cd "$repo_root/bin/copilot-dynamo"
export GH_REPO=${DEMO_REPO#https://github.com/}

scene=${1:?usage: demo.sh install|connect|model|result <task> <pr-number>}
case $scene in
  install)
    chapter 'Installed once, by the platform team'
    pe 'cat orka-values.yaml'
    nap 2
    pe 'kubectl get pods -n orka-system'
    aside 'orka-demo-oidc stands in for your identity provider in this recording.'
    nap 1.5
    ;;
  connect)
    chapter 'A developer joins'
    pe 'orka connect github'
    pe 'orka host status'
    nap 1.5
    ;;
  model)
    chapter 'The model moves inside the cluster'
    pe 'kubectl get nodes -L nvidia.com/gpu.product'
    pe 'kubectl get pods -n inference'
    pe 'orka models list'
    nap 1.5
    ;;
  result)
    task=${2:?usage: demo.sh result <task> <pr-number>}
    pr=${3:?usage: demo.sh result <task> <pr-number>}
    chapter 'An agent next to the model opened the pull request'
    pe 'orka task list --since 30m'
    # Orka, not the model, records which pull request the task opened.
    pe "orka task get $task | head -13"
    pe "gh pr diff $pr --name-only"
    # CI is GitHub's verdict, not the model's: wait for it before showing it.
    wait_for 'CI on the pull request' "gh pr checks $pr >/dev/null" 600
    pe "gh pr checks $pr"
    nap 2
    ;;
  *)
    echo "unknown scene: $scene" >&2
    exit 2
    ;;
esac
