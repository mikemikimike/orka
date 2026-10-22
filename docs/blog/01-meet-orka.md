---
title: "Part 1 — Meet Orka: your first agent in five minutes"
series: "Orka, piece by piece"
part: 1
status: draft
---

# Part 1 — Meet Orka: your first agent in five minutes

*Part 1 of 4 in "Orka, piece by piece." [Start with the series introduction](00-series-intro.md).*

Imagine you want an AI agent to review a repository for security problems. Today that usually
means someone installs a coding tool on their laptop, pastes in their personal API key, points
it at a checkout, and shares the result in a chat message. It works, once. Then a teammate
wants to do the same thing, and another, and the security team asks whose key that was.

Orka takes the same job and gives it a proper home. In this part we will:

1. install Orka,
2. run a first Task,
3. ask a model a question,
4. hand a real repository to a coding agent,
5. and look at all the ways people can start that same work.

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
ceremony for `echo`, but notice what you got for free: the work ran in its own Pod, it has a
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

First, store a model key in a Kubernetes Secret and create a Provider that points at it:

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

After a minute or so the Task reaches `Succeeded`, and you can read the answer from the API
or the CLI. The person who created this Task never saw the API key. That is the point.

## Step 4: Hand a repository to a coding agent

The second kind of Task runs a real coding agent: **Codex, Claude Code, GitHub Copilot CLI, or
OpenCode**. These are the same tools developers run in a terminal, now running in your cluster.

An Agent for a coding agent is short. It names the model and the runtime:

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

Swap `codex` for `claude`, `copilot`, or `opencode` and pick a matching model. Nothing else
in your setup changes. That is composability again: the runtime is a piece you can swap.

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

Orka clones the repository, gives the agent a working copy, and records what it does. The
first coding Task warms up a pool for that runtime, which takes a few minutes; after that,
new Tasks start in seconds.

Change `intent: read` to `intent: write` and add publication credentials, and the agent can
change the code and Orka will open the branch and pull request. Here is the part worth
remembering for Part 2: **the agent itself never holds a Git credential.** It only edits
files. A separate component checks the change and does the push.

(Coding agents reach their model through a small gateway you run, rather than through the
Provider from Step 3, so their model key also stays out of the agent's hands. The
[provider proxy guide](https://orka-agents.github.io/orka/docs/provider-proxy) sets it up once.)

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
  and tell me what it printed" — and an orchestrator creates and runs the Tasks for you.
- **The coding tools you already use.** Orka serves
  [OpenAI-compatible](https://orka-agents.github.io/orka/docs/openai-compat) and
  [Anthropic-compatible](https://orka-agents.github.io/orka/docs/anthropic-compat) endpoints.
  Point Claude Code, Codex, OpenCode, Cursor, or Continue at Orka, and nobody on the team
  needs their own provider key. (By default Orka adds its own tools and instructions to these
  requests; one header turns that off when your tool brings its own.)
- **Your team chat, your repository, your calendar.** Teams and Telegram messages, GitHub
  labels and pull requests, and cron schedules can all start Tasks. That is Part 3.

Whichever door someone uses, they end up with the same kind of Task, governed by the same
Agents and recorded the same way. A platform team sets things up once; everyone else uses
what they already know.

## What you have now

In five minutes and a few YAML files, you have:

- a place where AI work runs that is not someone's laptop,
- model keys that only the cluster can see,
- three kinds of work under one roof,
- a coding agent that can review or change a real repository without holding a Git credential,
- and a record of every Task.

## Next up

In **Part 2 — Trust by design**, we look at *why* it is safe to let these agents do real work:
where every credential lives, how a person can approve an action before it happens, how an
agent can act as you on GitHub without ever seeing your token, and how to run agents in
stronger sandboxes when you need them.

**Try it yourself:** [Getting started](https://orka-agents.github.io/orka/docs/getting-started) ·
[Dashboard demo](https://orka-agents.github.io/orka/demo/) ·
[Demo videos](https://orka-agents.github.io/orka/docs/demos)
