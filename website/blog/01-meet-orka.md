---
title: "Part 1 — Meet Orka: a workplace for AI agents"
slug: meet-orka
date: 2026-10-21T10:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "What Orka is, the three pieces everything is built from, the kinds of work it runs, and the many ways people reach it."
---

*Part 1 of 4 in "Orka, piece by piece." [Start with the series introduction](00-series-intro.md).*

Think about a new teammate's first day. Before they do anything useful, someone gives them a
desk, a badge that opens some doors and not others, accounts for the systems they need, a
manager who signs off on the big decisions, and somewhere to record what they did. Nobody
calls that bureaucracy. It is what lets someone new do real work without putting everyone
else at risk.

AI agents are joining a lot of teams right now, and most of them arrive with none of that.

<!-- truncate -->

Today an agent usually lives on one person's laptop or inside one person's account. It runs
with that person's keys and logins, it can reach whatever they can reach, and when it
finishes, the result gets pasted into a chat. That is fine for one person trying things out.
It falls apart when a team wants an agent to draft the week's posts for everyone to review,
answer customers' questions at night, or pick up issues overnight. Then the questions start:
whose key was that, what did the agent touch, and who said it could?

**Orka is a workplace for AI agents.** It runs on the Kubernetes cluster you already have.
Each piece of agent work gets a place to run, only the keys it needs, rules that do not depend on
the agent behaving, and a record of what happened. People reach that work from wherever they
already are: a chat app, a text message, a dashboard, a terminal, a pull request.

This part is the tour: what Orka is made of, what kinds of work it runs, and how people start
that work. If you would rather try it first, the
[getting started guide](https://orka-agents.github.io/orka/docs/getting-started) takes you
from an empty cluster to a first agent in about five minutes, with one Helm command to
install.

## Three pieces that combine

Everything in Orka is built from three building blocks:

| Building block | Think of it as | Example |
| --- | --- | --- |
| **Provider** | Which model service to call, and where its key lives | "Anthropic, key in the `anthropic-secret` Secret" |
| **Agent** | A job description | "Use Claude, be a helpful assistant, you may search the web" |
| **Task** | A work order | "Draft three taglines for our spring launch" |

They point at each other. A Task names an Agent, and an Agent names a Provider. Here is the
middle and the end of that chain (shortened; the getting started guide has the full files):

```yaml
kind: Agent
metadata:
  name: assistant
spec:
  providerRef:
    name: anthropic          # the Provider
  systemPrompt:
    inline: "You are a helpful assistant."
  tools:
    - name: web_search
---
kind: Task
metadata:
  name: hello-task
spec:
  type: ai
  agentRef:
    name: assistant          # the Agent
  prompt: "Draft three taglines for our spring launch."
```

You write each piece once and reuse it. Ten Tasks can share one Agent; five Agents can share
one Provider. And notice what the Task does *not* contain: a key. The model key lives in a
Kubernetes Secret (the cluster's built-in store for passwords and keys), usually put there
once by whoever runs Orka. Everyone who sends Tasks to `assistant` uses the model without
ever being handed the key.

That is the first sign of something you will see across the whole series: **Orka is a set of
small pieces that combine**, and you only add the ones you need.

## Three kinds of work

A Task can run one of three kinds of work:

| Task type | What runs | Good for |
| --- | --- | --- |
| `ai` | Orka's own AI worker, calling the model with tools | Assistants, research, drafting, summaries, coordinating other agents |
| `agent` | Codex, Claude Code, GitHub Copilot CLI, or OpenCode on a Git repository | Code review, bug fixes, pull requests |
| `container` | Any container image, no model involved | Reports, data exports, builds, scripts |

The `ai` kind is the one above, and it is where most agents live. The marketing assistant and
the personal assistant from the introduction are both `ai` agents: a model, a job description,
and tools. Orka has built-in tools such as web search, code execution, and fetching a web page,
and you can add your own, such as your calendar or your social media scheduler (Part 3 shows
how). The model decides when to use them.

The `agent` kind is for one use case Orka has gone especially deep on: software. These are the
same coding tools developers run in a terminal, now running in your cluster. A coding Task
names a repository and says whether the agent may only read it or may change it:

```yaml
spec:
  type: agent
  agentRef:
    name: codex-agent
  prompt: "Review this repo for security issues. Do not modify files."
  workspace:
    intent: read
    gitRepo: https://github.com/orka-agents/orka.git
```

Orka clones the repository, gives the agent its own working copy, and records what it does.
Which coding tool runs is a setting on the Agent, so switching from Codex to Claude Code is
mostly a one-line change (OpenCode asks for a couple more model settings), and the Tasks stay
the same. The tool is a piece you can swap.

The `container` kind sounds boring, and that is why it matters. Agents often need a report
generated, a data export, a build, or a script, and those are just containers. One platform runs all three kinds of
work with the same record and the same rules.

## Coding agents: three jobs, kept apart

For every agent, the model key stays in the cluster. Coding agents get a stricter arrangement,
because they work on your code and can run commands. Orka splits that work into three jobs
and gives each one to a different part:

| Job | Who does it | What it holds |
| --- | --- | --- |
| Talk to the model | The provider proxy, a small gateway in the cluster | The model key |
| Do the work | The agent, in its own private space | A copy of the code, plus short-lived passes that only work with Orka's own proxies; no model key, no Git credential |
| Deliver the result | The publisher | The Git credential, but no model access |

No single part holds everything, and the real model key and Git credential never reach the
agent. When you let an agent change code, it edits files in its own space and stops there.
The publisher checks the change, pushes it to a branch Orka chose, and, if you asked for one,
opens the pull request. Part 2 is about why that split matters so much.

## Built for agents that wait

A lot of agent work comes in bursts: a few minutes of work, then waiting for a person to reply,
for a sign-off, for automated checks to finish, or for the next message to arrive. Orka is
built so that waiting does not keep machines busy.

`ai` and `container` Tasks run as short-lived Jobs (Kubernetes' one-off runs of a container)
that end when their work does. Coding agents are slow to start from scratch, so Orka keeps a
warm *pool* for each one: a running Pod (Kubernetes' unit for running containers) that hosts
several agents' private sessions at once. The first coding Task starts the pool, which takes a
few minutes; after that, new Tasks start in seconds. The pool shuts down after it has been
empty for a while (15 minutes by default), so when nothing is running in it, it is not using
your cluster. You will see the same idea again in Part 3, where an agent can be put to
sleep between messages.

## Many ways in, one Task underneath

Writing YAML is one way in. Orka is built so people can start the same work from wherever
they already are:

- **The `orka` CLI**, for scripts and terminals.
- **The web dashboard**, built into Orka, for watching Tasks run, reading results, and
  approving actions. You can [try it in your browser](https://orka-agents.github.io/orka/demo/)
  on sample data.
- **Chat.** Describe what you want in plain language, and an orchestrator (an AI agent whose
  job is to run other Tasks) creates and runs them for you.
- **Your chat apps, your schedule, your repository.** Messages in Teams, Telegram, or
  iMessage, cron schedules, and GitHub labels and pull requests can all start Tasks. That is
  Part 3.
- **For developers, the coding tool already on their laptop.** More on this one below.

Whichever of these doors someone uses, they end up with the same kind of Task, governed by the
same Agents and recorded the same way. A platform team sets things up once; everyone else uses
what they already know.

### For developers: bringing your own coding tool

Earlier, Orka ran Codex *inside* the cluster. There is another way to bring the two together:
keep Claude Code, Codex, or OpenCode on your laptop, and point it at Orka instead of at a model
vendor. Orka speaks the same
[Anthropic-style](https://orka-agents.github.io/orka/docs/anthropic-compat) and
[OpenAI-style](https://orka-agents.github.io/orka/docs/openai-compat) interfaces those tools
already expect. The model key stays in the cluster, and each person signs in with an Orka
token instead.

Orka can play two very different roles here, and you choose with one setting, a request
header called `X-Orka-Tools`:

| | A shared model endpoint (`X-Orka-Tools: disabled`) | A coordinator (the default) |
| --- | --- | --- |
| What your tool can do | Keeps its own tools: it still reads and edits files and runs commands on your laptop (a few model options, such as extended thinking, are not passed through) | The model gets Orka's tools instead of your tool's: start agent Tasks in the cluster, check on them, cancel them, open pull requests, check a pull request's automated tests |
| What Orka does | Holds the key and passes model calls through | Holds the key *and* does the work in the cluster |
| Pick it when | You want your usual tool, minus the personal API key | Coding agents and repository access are set up, and you want to type "fix issue 42 and open a pull request" and let the cluster do it |

The first role is the easy win: everyone keeps their workflow, and the keys move into the
cluster. Those requests are plain model calls; they do not become Tasks. The second turns
your terminal into one more door to everything in this series: work it hands to agents
becomes ordinary Tasks, with the same records and rules. In that mode, the model no longer
gets your tool's file and shell tools; Orka runs its own tools in the cluster instead. Setup differs a little per tool, and the two guides
linked above cover what each one needs.

## The shape of it

Step back and here is what Orka gives you:

- a place where agent work runs that is not someone's laptop or someone's personal account,
- model keys that only the cluster can see,
- three kinds of work under one roof, built from three small pieces,
- many ways in, from a text message to a terminal, all leading to the same kind of record,
- and, for software teams, coding agents that can review or change a real repository without
  holding a Git credential.

## Next up

In **[Part 2 — Trust by design](02-trust-by-design.md)**, we look at *why* it is safe to let
these agents do real work: where every credential lives, what a fooled agent can and cannot
do, how a person can approve an action before it happens, and how to run agents in stronger
sandboxes when you need them.

**Try it yourself:** [Getting started](https://orka-agents.github.io/orka/docs/getting-started) ·
[Dashboard demo](https://orka-agents.github.io/orka/demo/) ·
[Demo videos](https://orka-agents.github.io/orka/docs/demos)
