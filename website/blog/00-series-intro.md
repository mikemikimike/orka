---
title: "Orka, piece by piece: a four-part introduction"
slug: orka-piece-by-piece
date: 2026-10-21T09:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "Orka runs AI agents on Kubernetes with the keys kept in the cluster. A four-part, beginner-friendly tour of what it is and how its pieces fit together."
---

AI agents are good at real work now. They can answer questions, fix bugs, review pull
requests, and open new ones. The hard part is no longer *whether* an agent can do the work.
It is everything around the work:

- Where does the agent run, and what can it touch?
- Whose API key is it using, and who else has a copy?
- What did it actually do yesterday, and how much did that cost?
- How does the rest of the team use it without everyone setting it up on their own laptop?

<!-- truncate -->

**Orka** is an open-source project that answers those questions by running AI agents on
Kubernetes. You describe a piece of work, Orka runs it somewhere isolated in your cluster,
keeps a record of what happened, and gives you the result. The model keys stay in the
cluster. People and automation get a scoped token instead.

This series is for people who have never used Orka. You do not need to be a Kubernetes
expert, but it helps to know that Kubernetes runs containers for you and that you talk to it
with `kubectl`. Each part stands on its own, so read them in order or jump to the one you
care about.

> **A note before we start:** Orka is experimental and moving quickly. Commands and fields
> in this series may change between releases. The
> [documentation](https://orka-agents.github.io/orka/docs) is always the source of truth.

## Two ideas to keep in mind

Two ideas come back in every part of this series.

**1. Orka is made of small pieces that snap together.** There are three building blocks you
will see everywhere:

| Building block | Plain-English meaning |
| --- | --- |
| **Provider** | *Which model, and where its key lives.* Anthropic, OpenAI, Azure OpenAI, or anything compatible. |
| **Agent** | *A job description.* Which model to use, what instructions to follow, which tools it may use. |
| **Task** | *A work order.* "Do this one thing now." |

Everything bigger — a schedule, a pull request reviewer, a team chat bot, a coordinator that
delegates to other agents — is built by combining these same three pieces with a few
optional extras. You start small and add only what you need.

**2. Orka meets you where you already work.** You do not have to adopt a new tool to use it.
The same Task can start from:

- `kubectl apply` with a YAML file, if you live in Kubernetes
- the `orka` command-line tool, if you like scripts
- a built-in web dashboard, if you prefer clicking
- a plain-language chat, if you just want to say what you need
- the AI coding tools you already use, such as Claude Code or Codex, pointed at Orka instead
  of at a model vendor
- a GitHub label on an issue, or a new pull request arriving
- a message in Microsoft Teams or Telegram, or a call from another agent
- a cron schedule that runs while you sleep

Behind every one of those doors is the same Task, with the same record and the same rules.

## The series

### Part 1 — Meet Orka: your first agent in five minutes

**The question it answers:** *What is Orka, and what does using it feel like?*

We install Orka with one Helm command, run a first Task, ask a model a question, and then hand
a real repository to a coding agent. Along the way, we meet the three building blocks and the
three kinds of work a Task can do.

Features we cover:

- **The three Task types:** `ai` (ask a model, with tools like web search and code execution),
  `agent` (run Codex, Claude Code, GitHub Copilot CLI, or OpenCode on a git repository), and
  `container` (run any command, no model needed)
- **Provider, Agent, Task**, and how one points at the next
- **Coding agents that open pull requests**, without the agent ever holding a Git credential
- **Many front doors:** `kubectl`, the `orka` CLI, the dashboard, chat, and the
  OpenAI- and Anthropic-compatible endpoints for tools like Claude Code
- **The dashboard demo**, which runs in your browser with no install

### Part 2 — Trust by design: keys, approvals, and sandboxes

**The question it answers:** *Why is it safe to let agents do real work?*

An agent reads untrusted text all day: issues, web pages, code from strangers. Part 2 explains
how Orka keeps that agent away from the things it should not have, and how you decide how
much control you want, from "just keep the keys safe" to "a person approves every write."

Features we cover:

- **Keys stay in the cluster:** model keys live in Kubernetes Secrets; people, CI, and tools get
  scoped tokens
- **"Thinks but cannot push":** coding agents have no Git credentials; a separate publisher
  checks and pushes their changes
- **Human approval** for actions that matter, with Orka running exactly the action a person
  approved
- **Connectors**, so an agent can act as *you* on GitHub, with approval on every write
- **Outbound access policies and transaction tokens** for teams that need tighter control
- **Stronger sandboxes** with execution workspaces (Agent Sandbox and Agent Substrate) that
  suspend, resume, and checkpoint an agent's files

### Part 3 — Agents that keep working: automation, teamwork, and your own tools

**The question it answers:** *How do I go from one Task to agents that are part of how my team works?*

This is where composability pays off. We take the same building blocks from Part 1 and add
schedules, repository automation, teams of agents, memory, and custom tools.

Features we cover:

- **Scheduled Tasks:** any Task on a cron schedule
- **Repository monitors:** review every pull request as it arrives
- **Issue to pull request:** label an issue `orka:implement` and Orka plans, implements, reviews,
  and repairs the PR
- **Security scanning** of a repository with validated findings
- **Multi-agent coordination:** one agent delegates to specialists, inside limits you set
- **Autonomous Tasks** that plan, act, and re-plan toward a goal
- **Memory** that agents propose and people approve
- **Gateways** for Microsoft Teams, Telegram, and agent-to-agent (A2A) requests
- **Your own tools:** HTTP and MCP tools, toolboxes of extra command-line programs, and your own
  agent runtimes (for example AgentKit or Microsoft Foundry agents)

### Part 4 — Running Orka for real: visibility, cost, and how it works inside

**The question it answers:** *How do I operate this for a team and trust what it tells me?*

The last part is for the person who will keep Orka running. We look at what Orka records,
how to measure cost, and how the system is built underneath.

Features we cover:

- **The durable record:** every Task's phase, result, artifacts, and event timeline
- **OpenTelemetry tracing and GenAI metrics** in the observability tools you already run
- **Usage and PR outcomes:** tokens per merged pull request, per team
- **The dashboard** for watching and steering work
- **How it works inside:** the controller, warm RuntimePools that scale to zero, digest-pinned
  images, and the clean-room publisher
- **Day-two operations:** install, upgrade, and where to look when something is stuck

## Try it before reading further

If you want a feel for Orka before Part 1, open the
[dashboard demo](https://orka-agents.github.io/orka/demo/). It runs the real dashboard in your
browser on sample data, with a guided tour, and sends nothing anywhere. The
[demo videos](https://orka-agents.github.io/orka/docs/demos) show the same features on a live
cluster.

See you in Part 1.
