# The Copilot app, with Kubernetes as the backend

A platform team installs Orka once with Helm. A developer runs
`orka connect github` and gets a personal agent host in the cluster, then picks
that host in the Copilot app and asks for a change; the session runs on the
cluster with Copilot's own model. Then the model moves inside too: Qwen3 Coder
runs on NVIDIA Dynamo on an A100 node, Orka is added to the Copilot app as a
model provider, and Orka's coordinator hands the change to an agent that runs
next to the model. Orka records the pull request, and GitHub runs the check.

The target repository ([sozercan/orka-demo-inventory](https://github.com/sozercan/orka-demo-inventory))
and the requests are synthetic. Sessions, Tasks, pull requests, and checks come
from execution. `orka-demo-oidc` stands in for the identity provider.

## What runs where

| Step | Runs as | Where |
| --- | --- | --- |
| Install | Helm release `orka` from main, values in `orka-values.yaml` | AKS, `orka-system` |
| Developer joins | `orka connect github`: a GitHub link and the developer's `copilotd` agent host | AKS, `orka-agent-hosts` |
| Copilot app on the cluster | Copilot app session in the host's environment; the model is Copilot's | host Pod |
| In-cluster model | DynamoGraphDeployment `inference/qwen3-coder` (Frontend and vLLM worker, Qwen3-Coder-30B-A3B FP8) behind Orka's provider proxy | A100 node |
| Copilot app with Orka as model provider | Local session; custom endpoint `<orka>/openai/v1`, model `dynamo/qwen3-coder` (Orka's coordinator); Agent `qwen-coder` runs the change | coordinator in the controller, agent Task in the cluster |

## Prepare

`demo.sh` sources `ORKA_DEMO_ENV` (default `bin/copilot-dynamo/env.sh`, not
committed). It puts the `orka` CLI built from main on `PATH`, keeps its config
(a person's OIDC token, not a ServiceAccount token) under `ORKA_CONFIG_DIR`,
pins the kube context through `KUBECONFIG`, and sets `ORKA_API` and
`DEMO_REPO`. The values file shown on camera sits next to it.

Orka main pins `copilotd` 0.9.6, which speaks AHP 0.9 only; the current Copilot
app offers only AHP 1.0 and fails to connect with `unsupported protocol
version`. This recording set `controller.agentHosts.template.copilotdImage` to
`ghcr.io/github/copilotd:1.0.5` in the image pins. `AgentHostTemplate` is
immutable, so delete it before `helm upgrade`; the host rolls on its PVC and
keeps its environment ID.

The in-cluster scene also needs the `qwen-coder` Agent and a `github-credentials`
Secret (key `token`) in the Task namespace, and the Copilot app's custom
endpoint must reach Orka from the laptop (a port-forward), so that session runs
on "This computer".

## Record

Terminal scenes, one cast each (the `result` scene takes the Task and PR from
the in-cluster take):

```sh
asciinema rec demo/casts/15-copilot-app-dynamo-<scene>.cast --overwrite --headless --return \
  --window-size 100x28 --idle-time-limit 2 --command 'bash demo/15-copilot-app-dynamo/demo.sh <scene>'
python3 demo/lib/markers.py demo/casts/15-copilot-app-dynamo-<scene>.cast
python3 demo/lib/typing.py demo/casts/15-copilot-app-dynamo-<scene>.cast
```

The Copilot app and browser clips are recordings of one window each
(`screencapture -v -l <window-id>`); the browser uses its own new window.
Record the browser while recording `connect`, so the consent and the terminal
share a clock. The browser briefly shows the consumed `#completion=` value in
its address bar; mask it in the edit.

## What the takes showed

- The Copilot app session asked for approval before an unrequested README edit.
  The host has no Git author configured, so the agent committed with the
  repository's existing one. PR #28.
- The coordinator's first in-cluster take created the Task and opened PR #29,
  then invented a reply: PR "#123", two reviews, six passing checks.
  `wait_for_task` and `fetch_task_output` do not return the pull request a Task
  opened (it is in `status.delivery.prReceipt`), so the recorded take asks only
  for the Task name and phase, and the terminal shows the PR from Orka's record.
  Its `create_agent_task` calls failed three times before one succeeded. PR #30;
  #29 is closed.
- Both coordinator PRs are titled "You are an expert software engineer. Your
  task is to implement the requested change.": the publisher titles a PR from
  the first line of the Task prompt, which is the coordinator's template.
- The coordinator's reply ends with a literal `</ORKA_GOAL_STATE_REACHED>`.

## Checks without a cluster

```sh
bash -n demo/15-copilot-app-dynamo/demo.sh
shellcheck -x demo/15-copilot-app-dynamo/demo.sh
```
