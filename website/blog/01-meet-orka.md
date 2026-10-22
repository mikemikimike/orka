---
title: "Part 1 — Meet Orka: your first agent in five minutes"
slug: meet-orka
date: 2026-10-21T10:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "Install Orka, run a first Task, ask a model, and hand a repository to a coding agent, all in about five minutes."
---

*Part 1 of 4 in "Orka, piece by piece." [Start with the series introduction](00-series-intro.md).*

Imagine you want an AI agent to review a repository for security problems. Today that usually
means someone installs a coding tool on their laptop, pastes in their personal API key, points
it at a checkout, and shares the result in a chat message. It works, once. Then a teammate
wants to do the same thing, and another, and the security team asks whose key that was.

<!-- truncate -->

Orka takes the same job and gives it a proper home. In this part we will:

1. install Orka and run a first Task (this part really does take about five minutes),
2. ask a model a question,
3. hand a real repository to a coding agent (this needs one extra setup step, so budget a
   little longer),
4. and look at all the ways people can start that same work.

## What is Orka, in one paragraph

Orka runs AI work on Kubernetes. You describe a piece of work as a **Task**. Orka runs it
somewhere isolated in your cluster, keeps a record of what happened, and gives you the result
through a REST API, a command-line tool, a web dashboard, or the chat and coding tools you
already use. The model keys stay in the cluster.

## Step 1: Install

You need a Kubernetes cluster, `kubectl`, and Helm. A local [kind](https://kind.sigs.k8s.io/)
cluster on your laptop is fine, and you do not need a model API key yet.

```bash
helm repo add orka https://orka-agents.github.io/orka/charts
helm repo update orka
helm install orka orka/orka --namespace orka-system --create-namespace --wait --timeout 10m
```

That is the whole install.

## Step 2: Your first Task (no AI yet)

The simplest kind of Task runs a command in a container. No model is involved:

```bash
kubectl -n orka-system apply -f - <<'EOF'
apiVersion: core.orka.ai/v1alpha1
kind: Task
metadata:
  name: hello
spec:
  type: container
  command: ["sh", "-c", "echo hello from orka"]
EOF

kubectl -n orka-system get task hello --watch
```

When the phase says `Succeeded`, Orka has run its first Task. That might look like a lot of
ceremony for `echo`, but notice what you got for free: the work ran in its own Pod (Kubernetes'
unit for running a container), it has a
name you can look up later, and it has a status you can check from anywhere.

> **Tip:** create everything in the `orka-system` namespace. Orka watches one namespace, and
> things created elsewhere are quietly ignored.

## The three building blocks

Before we bring in a model, meet the three pieces you will use in every Orka setup:

| Building block | Think of it as | Example |
| --- | --- | --- |
| **Provider** | Which model service to call, and where its key lives | "Anthropic, key in the `anthropic-secret` Secret" |
| **Agent** | A job description | "Use Claude, be a helpful assistant, you may search the web" |
| **Task** | A work order | "Answer: what is Kubernetes?" |

A Task points at an Agent. An Agent points at a Provider. You write each one once and reuse it.
Ten Tasks can share one Agent; five Agents can share one Provider. This is the first sign of
something you will see across the whole series: **Orka is a set of small pieces that combine.**

## Step 3: Ask a model

First, store a model key in a Kubernetes Secret (the cluster's built-in store for passwords and
keys) and create a Provider that points at it:

```bash
kubectl -n orka-system create secret generic anthropic-secret \
  --from-literal=api-key=your-api-key

kubectl apply -f - <<'EOF'
apiVersion: core.orka.ai/v1alpha1
kind: Provider
metadata:
  name: anthropic
  namespace: orka-system
spec:
  type: anthropic
  secretRef:
    name: anthropic-secret
    key: api-key
  defaultModel: claude-opus-5
EOF
```

OpenAI, Azure OpenAI, and any OpenAI-compatible server (including a model running inside your
cluster) work the same way. No API key at all? A GitHub Copilot subscription is enough when
paired with the [Vekil](https://github.com/sozercan/vekil) model gateway; the
[getting started guide](https://orka-agents.github.io/orka/docs/getting-started) shows how.

Next, an Agent — the job description:

```bash
kubectl apply -f - <<'EOF'
apiVersion: core.orka.ai/v1alpha1
kind: Agent
metadata:
  name: assistant
  namespace: orka-system
spec:
  providerRef:
    name: anthropic
  systemPrompt:
    inline: "You are a helpful assistant."
  tools:
    - name: web_search
EOF
```

`web_search` is one of Orka's built-in tools. Others include code execution, fetching a web
page, and reading and writing files. The model decides when to use them.

Finally, the work order:

```bash
kubectl apply -f - <<'EOF'
apiVersion: core.orka.ai/v1alpha1
kind: Task
metadata:
  name: hello-task
  namespace: orka-system
spec:
  type: ai
  agentRef:
    name: assistant
  prompt: "What is Kubernetes?"
EOF
```

After a minute or so the Task reaches `Succeeded`. To read the answer, connect to Orka's API
(a `kubectl port-forward` and a short-lived token, covered in
[Connect to the API](https://orka-agents.github.io/orka/docs/getting-started#connect-to-the-api)),
then ask for the result:

```bash
curl -H "Authorization: Bearer ${ORKA_TOKEN}" \
  http://localhost:8080/api/v1/tasks/hello-task/result
```

If you [install the `orka` CLI](https://orka-agents.github.io/orka/docs/getting-started#the-cli),
the same thing is `orka task result hello-task`.

Here you created the key yourself, but on a team that is usually done once, by an
administrator. After that, anyone can send Tasks to the `assistant` Agent without ever being
handed the model key. That is the point.

## Step 4: Hand a repository to a coding agent

The second kind of Task runs a real coding agent: **Codex, Claude Code, GitHub Copilot CLI, or
OpenCode**. These are the same tools developers run in a terminal, now running in your cluster.

**First, one setup step.** Coding agents do not use the Provider from Step 3. They reach their
model through a small gateway you run, so the model key never enters the agent's process.
Follow the [provider proxy guide](https://orka-agents.github.io/orka/docs/provider-proxy)
once, and check that the proxy is ready before you continue:

```bash
kubectl -n orka-system get deploy -l app.kubernetes.io/component=provider-auth-proxy
```

Now the Agent. For a coding agent it is short: it names the model (one your gateway offers)
and the runtime:

```yaml
apiVersion: core.orka.ai/v1alpha1
kind: Agent
metadata:
  name: codex-agent
  namespace: orka-system
spec:
  model:
    name: gpt-6-astra
  runtime:
    type: codex
```

Swap `codex` for `claude`, `copilot`, or `opencode` and pick a matching model. (OpenCode
asks for a little more: a `provider/model` name and explicit `model.contextWindow` and
`model.maxTokens` values.) Your Tasks do not change. That is composability again: the runtime
is a piece you can swap.

Now a Task that reviews a public repository without changing anything:

```yaml
apiVersion: core.orka.ai/v1alpha1
kind: Task
metadata:
  name: code-review
  namespace: orka-system
spec:
  type: agent
  agentRef:
    name: codex-agent
  prompt: "Review this repo for security issues. Do not modify files."
  workspace:
    intent: read
    gitRepo: "https://github.com/orka-agents/orka.git"
    branch: main
```

Orka clones the repository, gives the agent a working copy, and records what it does.

Coding agents are slow to start from scratch, so Orka keeps a *pool* for each one: a running
Pod that can host several agents' private sessions at once and shuts down when nobody is using
it. The first coding Task starts the pool, which takes a few minutes; after that, new Tasks
start in seconds.

To let an agent change code, create a new Task (on a repository you can push to) with
`intent: write` and a Git credential for publishing, and Orka will publish the change to a
branch. Add `createPR: true`, the branch to merge into, and a GitHub API credential, and Orka
opens the pull request too. Here is the part worth remembering for
Part 2: **the agent itself never holds a Git credential.** It only edits files. A separate
component checks the change and does the push.

## The third type: plain containers

We already used it in Step 2. `type: container` runs any image with any command. It sounds
boring, and that is why it matters: agents often need a build, a test run, or a script, and
those are just containers. One platform runs all three kinds of work, with the same record
and the same rules.

| Task type | What runs | Good for |
| --- | --- | --- |
| `ai` | Orka's own AI worker, calling the model with built-in tools | Questions, research, summaries, coordinating other agents |
| `agent` | Codex, Claude Code, Copilot CLI, or OpenCode on a git repository | Code review, bug fixes, pull requests |
| `container` | Any container image | Builds, tests, scripts |

## Many ways in, one Task underneath

So far we used `kubectl` and YAML. That is one way in. Orka is built so people can start the
same work from wherever they already are:

- **The `orka` CLI.** `orka task list`, `orka task result hello-task`, `orka task status
  code-review`. Good for scripts and terminals.
- **The web dashboard.** Built into Orka, turned on with one Helm value. Watch Tasks run,
  read results, approve actions. You can
  [try it in your browser](https://orka-agents.github.io/orka/demo/) on sample data first.
- **Chat.** Describe what you want in plain language — "run a container that prints the date
  and tell me what it printed" — and an orchestrator (an AI agent whose job is to run other
  Tasks) creates and runs them for you.
- **The coding tools you already use.** Orka serves
  [OpenAI-compatible](https://orka-agents.github.io/orka/docs/openai-compat) and
  [Anthropic-compatible](https://orka-agents.github.io/orka/docs/anthropic-compat) endpoints.
  Point Claude Code, Codex, OpenCode, Cursor, or Continue at Orka, and nobody on the team
  needs their own provider key. This door works a little differently from the others: the tool
  keeps running on your laptop, and Orka serves as its shared model endpoint, so these requests
  do not necessarily become Tasks. One thing to know: by default Orka swaps your tool's own
  tools for Orka's server-side ones. To keep your tool working exactly as it does today, with its
  tools running on your laptop, send the header `X-Orka-Tools: disabled`.
- **Your team chat, your repository, your calendar.** Teams and Telegram messages, GitHub
  labels and pull requests, and cron schedules can all start Tasks. That is Part 3.

Apart from the coding-tool endpoints, whichever door someone uses, they end up with the same
kind of Task, governed by the same Agents and recorded the same way. A platform team sets things up once; everyone else uses
what they already know.

## What you have now

In five minutes and a few YAML files, you have:

- a place where AI work runs that is not someone's laptop,
- model keys that only the cluster can see,
- three kinds of work under one roof,
- a coding agent that can review or change a real repository without holding a Git credential,
- and a record of every Task.

## Next up

In **[Part 2 — Trust by design](02-trust-by-design.md)**, we look at *why* it is safe to let these agents do real work:
where every credential lives, how a person can approve an action before it happens, how an
agent can act as you on GitHub without ever seeing your token, and how to run agents in
stronger sandboxes when you need them.

**Try it yourself:** [Getting started](https://orka-agents.github.io/orka/docs/getting-started) ·
[Dashboard demo](https://orka-agents.github.io/orka/demo/) ·
[Demo videos](https://orka-agents.github.io/orka/docs/demos)
