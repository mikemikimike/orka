---
title: "Orka, piece by piece: a four-part introduction"
slug: orka-piece-by-piece
date: 2026-10-21T09:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "Orka runs AI agents on Kubernetes with the keys kept in the cluster. A four-part, beginner-friendly tour of what it is and how its pieces fit together."
---

AI agents have changed jobs. Not long ago they answered questions in a chat window. Now they
fix bugs, review pull requests, triage issues, and open pull requests of their own. The hard
part is no longer *whether* an agent can do the work. It is everything around the work:

- Where does the agent run, and what can it touch?
- Whose API key is it using, and who else has a copy?
- What did it actually do yesterday, and how much did that cost?
- How does the rest of the team use it without everyone setting it up on their own laptop?

<!-- truncate -->

## Where this is going

A few things look likely over the next couple of years.

- **Agents move from the chat window into how teams work.** Not one person asking one agent a
  question, but agents reviewing every pull request, picking up labeled issues, running the
  nightly checks, and answering in the team chat.
- **There will be many kinds of agents, not one.** Well-known coding tools like Codex and
  Claude Code, agents hosted on cloud platforms, and small agents your own team builds for a
  single job. The models underneath them will keep changing every few months.
- **Agents act for people.** They comment on an issue as you, file a ticket for you, and reach
  the systems you can reach. *Who asked, and on whose behalf* starts to matter as much as which
  model ran.
- **Agents wait a lot.** They work in bursts, then wait on a person, on automated checks, or on
  the next message. Paying for an agent to sit idle makes little sense.

Put together, agents start to look less like a tool you open and more like coworkers. And
coworkers need a workplace: somewhere to work, a badge that opens the right doors, rules,
someone to sign off on big decisions, and a record of what got done.

**Orka is an open-source workplace for AI agents, built on Kubernetes.** You describe a piece
of work, Orka runs it somewhere isolated in your cluster, keeps a record of what happened, and
gives you the result. The model keys stay in the cluster; people and automation get a scoped
token instead.

Orka's bet is simple: the agents and models will keep changing, so the workplace around them
should stay steady. Whichever agent runs, the work is the same kind of Task, with the same keys
kept safe, the same rules, and the same record.

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

Most of those doors lead to the same Task, with the same record and the same rules. The
coding-tool door is a little different: there, Orka acts as a shared model endpoint, so your
team stops handing out provider keys, but the tool itself keeps running where it already runs.
If you want, Orka can also swap in its own tools, so the tool starts Tasks in the cluster for you.

Meeting you where you are goes for infrastructure too. Orka runs on the Kubernetes cluster you
already have, and when you want stronger sandboxes, it plugs into the sandbox projects you
choose instead of bringing its own.

You will also notice two habits running through every part. Orka **keeps jobs apart**: for a
coding agent, the part that talks to the model, the part that does the work, and the part that
delivers the result each hold only what they need, and the real model key and Git credential
stay out of the agent's reach. And Orka **plans for agents that wait**: much of an agent's work
is waiting on a person or on automated checks, so Orka can shut down or put to sleep the parts
that have nothing to do.

## The series

### [Part 1 — Meet Orka: a workplace for AI agents](01-meet-orka.md)

**The question it answers:** *What is Orka, and what does using it feel like?*

A tour of what Orka is made of: the three building blocks, the three kinds of work a Task can
do, how a coding agent works on a repository without holding the keys, and the many ways
people reach that work, including the coding tool already on their laptop.

Features we cover:

- **The three Task types:** `ai` (ask a model, with tools like web search and code execution),
  `agent` (run Codex, Claude Code, GitHub Copilot CLI, or OpenCode on a git repository), and
  `container` (run any command, no model needed)
- **Provider, Agent, Task**, and how one points at the next
- **Coding agents that open pull requests**, without the agent ever holding a Git credential
- **Three jobs, kept apart:** talking to the model, doing the work, and delivering the result
- **Many front doors:** `kubectl`, the `orka` CLI, the dashboard, and chat
- **Using Orka from Claude Code and other coding tools** on your laptop, either as a shared
  model endpoint that keeps the keys, or as a coordinator that starts work in the cluster
- **The dashboard demo**, which runs in your browser with no install

### [Part 2 — Trust by design: keys, approvals, and sandboxes](02-trust-by-design.md)

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
- **A walk through an attack:** what a fooled agent finds when it goes looking
- **Calling your own systems without handing agents the keys**
- **Stronger walls** around agents, from sandboxed Pods to a sandbox per conversation

### [Part 3 — Agents that keep working: automation, teamwork, and your own tools](03-agents-that-keep-working.md)

**The question it answers:** *How do I go from one Task to agents that are part of how my team works?*

This is where composability pays off. We take the same building blocks from Part 1 and add
schedules, repository automation, teams of agents, memory, and custom tools.

Features we cover:

- **Scheduled Tasks:** any Task on a cron schedule
- **Repository monitors:** review every pull request as it arrives
- **Issue to pull request:** label an issue `orka:implement` and Orka plans, implements, reviews,
  and repairs the PR, and `orka:pause` to hold it back
- **Security scanning** of a repository with validated findings
- **Multi-agent coordination:** one agent delegates to specialists, inside limits you set
- **Autonomous Tasks** that plan, act, and re-plan toward a goal
- **Memory** that agents propose and people approve
- **Sessions:** agents that continue a conversation, including one you started with Codex on
  your laptop and imported into Orka
- **Workspaces:** a sandbox per conversation (Agent Sandbox or Agent Substrate) that can suspend
  between Tasks and resume later; Agent Substrate can also save checkpoints and restore or fork
  from them
- **Three kinds of remembering:** the conversation, the files, and approved facts, each with
  its own owner
- **Gateways** for Microsoft Teams, Telegram, and agent-to-agent (A2A) requests
- **Your team's ways and your own tools:** skills and a persona, connecting your APIs, adding extra programs for coding agents
  (toolboxes), and packaging an agent you built yourself (AgentKit)

### [Part 4 — Running Orka for real: visibility, cost, and how it works inside](04-running-orka-for-real.md)

**The question it answers:** *How do I operate this for a team and trust what it tells me?*

The last part is for the person who will keep Orka running. We look at what Orka records,
how to measure cost, and how the system is built underneath.

Features we cover:

- **The durable record:** every Task's phase, result, artifacts, and event timeline
- **Tracing and metrics** in the monitoring tools you already run
- **Usage and PR outcomes:** tokens per merged pull request, per team
- **The dashboard** for watching and steering work
- **Backup model providers** for when one has an outage
- **One shared address** for every team's coding tools
- **How it works inside:** the one program that runs everything, warm pools of coding agents
  that shut down when idle, pinned images, and what happens when things restart
- **Day-two operations:** install, upgrade, and where to look when something is stuck

## Try it before reading further

If you want a feel for Orka before Part 1, open the
[dashboard demo](https://orka-agents.github.io/orka/demo/). It runs the real dashboard in your
browser on sample data, with a guided tour, and sends nothing anywhere. The
[demo videos](https://orka-agents.github.io/orka/docs/demos) show the same features on a live
cluster.

See you in [Part 1](01-meet-orka.md).
