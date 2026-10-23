---
title: "Orka, piece by piece: a four-part introduction"
slug: orka-piece-by-piece
date: 2026-10-21T09:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "Orka is an open-source workplace for AI agents, built on Kubernetes. A four-part, beginner-friendly tour of what it is and how its pieces fit together."
---

AI agents have changed jobs. Not long ago they answered questions in a chat window. Now they
do things. [OpenClaw](https://github.com/openclaw/openclaw) is an open-source personal
assistant you run yourself and talk to through chat apps like WhatsApp and Telegram; it
handles email, calendars, and files. OpenAI's Dots keep working in the background between
chats, each on a computer of its own. Meta's Muse drafts a small business's posts and ad
campaigns, and checks with the owner before publishing or spending anything. And coding
agents fix bugs and open pull requests of their own.

The hard part is no longer *whether* an agent can do the work. It is everything around the
work:

- Where does the agent run, and what can it touch?
- Whose keys and accounts is it using, and who else has a copy?
- What did it actually do yesterday, and how much did that cost?
- How does a whole team use it, without everyone setting it up on their own laptop?

<!-- truncate -->

## Where this is going

A few things look likely over the next couple of years.

- **Agents move from the chat window into how people work.** Not one person asking one agent a
  question, but agents drafting the week's posts, answering texts at night, triaging the
  support inbox, and reviewing every pull request.
- **There will be many kinds of agents, not one.** Personal assistants, business agents for
  marketing and support, coding tools like Codex and Claude Code, agents hosted on cloud
  platforms, and small agents your own team builds for a single job. The models underneath
  them will keep changing every few months.
- **Agents act for people.** They send an email as you, post for your business, and comment on
  an issue in your name. *Who asked, and on whose behalf* starts to matter as much as which
  model ran.
- **Agents wait a lot.** They work in bursts, then wait on a person, a sign-off, or the next
  message. Paying for an agent to sit idle makes little sense.

Put together, agents start to look less like a tool you open and more like coworkers. And
coworkers need a workplace: somewhere to work, a badge that opens the right doors, rules,
someone to sign off on big decisions, and a record of what got done.

**Orka is an open-source workplace for AI agents, built on Kubernetes.** You describe a piece
of work, Orka runs it somewhere isolated in your cluster, keeps a record of what happened, and
gives you the result. The model keys stay in the cluster; people and automation get a scoped
token instead.

Some of the agents above are run by a vendor for one person; others run on one person's own
laptop. Orka is for the next step: running agents like these for a whole team or company, on
infrastructure you control, with the keys out of the agent's reach, sign-off where it
matters, and a record of everything.

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

## Three agents we will follow

To keep things concrete, the series follows three agents:

- **A marketing assistant** for a small team. It drafts the week's posts from a product brief,
  checks last week's numbers, and asks a person before it publishes anything.
- **A personal assistant** you text from your phone. It keeps track of the conversation,
  remembers what you told it to remember, and works with your calendar and notes through tools
  you connect.
- **A coding agent** that reviews and changes code.

Coding is the use case Orka has gone deepest on, so it gets its own clearly marked sections.
Everything else in the series applies to any agent.

## Two ideas to keep in mind

Two ideas come back in every part of this series.

**1. Orka is made of small pieces that snap together.** There are three building blocks you
will see everywhere:

| Building block | Plain-English meaning |
| --- | --- |
| **Provider** | *Which model, and where its key lives.* Anthropic, OpenAI, Azure OpenAI, or anything compatible. |
| **Agent** | *A job description.* Which model to use, what instructions to follow, which tools it may use. |
| **Task** | *A work order.* "Do this one thing now." |

Everything bigger — a weekly schedule, an assistant you text, a coordinator that delegates to
other agents, a pull request reviewer — is built by combining these same three pieces with a
few optional extras. You start small and add only what you need.

**2. Orka meets you where you already work.** You do not have to adopt a new tool to use it.
The same Task can start from:

- a message in Microsoft Teams, Telegram, or iMessage
- a plain-language chat, if you just want to say what you need
- a built-in web dashboard, if you prefer clicking
- a cron schedule that runs while you sleep
- another agent's request
- `kubectl apply` with a YAML file, or the `orka` command-line tool
- for developers, a GitHub label on an issue, a new pull request, or the coding tools they
  already use, such as Claude Code or Codex, pointed at Orka instead of at a model vendor

Most of those doors lead to the same Task, with the same record and the same rules. The
coding-tool door is a little different: there, Orka acts as a shared model endpoint, so your
team stops handing out provider keys, but the tool itself keeps running where it already runs.
If you want, Orka can also swap in its own tools, so the tool starts Tasks in the cluster for you.

Meeting you where you are goes for infrastructure too. Orka runs on the Kubernetes cluster you
already have, and when you want stronger sandboxes, it plugs into the sandbox projects you
choose instead of bringing its own.

You will also notice two habits running through every part. Orka **keeps credentials away
from agents**: the keys for the tools and accounts an agent uses stay with Orka, and for
coding agents even the model key and the Git credential stay out of reach. And Orka **plans for
agents that wait**: much of an agent's work is waiting on a person or a sign-off, so Orka can
shut down or put to sleep the parts that have nothing to do.

## The series

### [Part 1 — Meet Orka: a workplace for AI agents](01-meet-orka.md)

**The question it answers:** *What is Orka, and what does using it feel like?*

A tour of what Orka is made of: the three building blocks, the three kinds of work a Task can
do, and the many ways people reach that work. Coding agents get a closer look at how they work
on a repository without holding the keys.

Features we cover:

- **Provider, Agent, Task**, and how one points at the next
- **The three Task types:** `ai` (a model with tools, where most agents live), `agent` (Codex,
  Claude Code, GitHub Copilot CLI, or OpenCode on a Git repository), and `container` (run any
  command, no model needed)
- **Built for agents that wait:** short-lived runs, and warm pools that shut down when idle
- **Many front doors:** Teams, Telegram, and iMessage, the dashboard, plain-language chat,
  schedules, the `orka` CLI, and `kubectl`
- **For coding agents:** three jobs kept apart, so the agent never holds the model key or a Git
  credential, and using Orka from the coding tool already on a developer's laptop
- **The dashboard demo**, which runs in your browser with no install

### [Part 2 — Trust by design: keys, approvals, and sandboxes](02-trust-by-design.md)

**The question it answers:** *Why is it safe to let agents do real work?*

An agent reads untrusted text all day: emails and messages from strangers, web pages,
documents, issues. Part 2 explains how Orka keeps that agent away from the things it should
not have, and how you decide how much control you want, from "just keep the keys safe" to "a
person approves every post."

Features we cover:

- **Keys stay in the cluster:** model keys live in Kubernetes Secrets; people and tools get
  scoped tokens
- **Human approval** for actions that matter, with Orka running exactly the action a person
  approved
- **Calling your own systems without handing agents the keys**
- **Connectors**, so an agent can act as *you*, with approval on every write (GitHub first)
- **Walking through an attack, twice:** what a fooled marketing assistant and a fooled coding
  agent can and cannot do
- **For coding agents:** "thinks but cannot push," with a separate publisher that checks and
  pushes their changes
- **Stronger walls** around agents, from sandboxed Pods to a sandbox per conversation

### [Part 3 — Agents that keep working: schedules, conversations, teamwork, and your own tools](03-agents-that-keep-working.md)

**The question it answers:** *How do I go from one Task to agents that are part of how people work?*

This is where composability pays off. We take the same building blocks from Part 1 and follow
the three agents as they gain schedules, chat apps, memory, teammates, and tools.

Features we cover:

- **Scheduled Tasks:** any Task on a cron schedule
- **Gateways** for Microsoft Teams, Telegram, iMessage, and agent-to-agent (A2A) requests
- **Sessions and memory:** agents that continue a conversation, and facts they propose and
  people approve
- **Multi-agent coordination:** one agent delegates to specialists, inside limits you set
- **Autonomous Tasks** that work toward a goal, and stop to ask before a step that needs
  sign-off
- **Your ways and your tools:** skills and a persona, connecting your apps and APIs, and
  packaging an agent you built yourself (AgentKit)
- **For software teams:** reviewing every pull request, turning labeled issues into pull
  requests, security scanning, continuing a laptop Codex conversation in the cluster, a
  sandbox per conversation that can suspend and resume (with checkpoints, restore, and fork on
  Agent Substrate), and extra programs for coding agents (toolboxes)

### [Part 4 — Running Orka for real: visibility, cost, and how it works inside](04-running-orka-for-real.md)

**The question it answers:** *How do I operate this for a team and trust what it tells me?*

The last part is for the person who will keep Orka running. We look at what Orka records,
how to see what agents cost, and how the system is built underneath.

Features we cover:

- **The durable record:** every Task's phase, result, artifacts, and event timeline
- **Tracing and metrics** in the monitoring tools you already run
- **Usage:** tokens per team, and for software teams, tokens per merged pull request
- **The dashboard** for watching and steering work
- **Backup model providers** for when one has an outage
- **One shared address** for every team's tools
- **How it works inside:** the one program that runs everything, warm pools that shut down
  when idle, pinned images, and what happens when things restart
- **Day-two operations:** install, upgrade, and where to look when something is stuck

## Try it before reading further

If you want a feel for Orka before Part 1, open the
[dashboard demo](https://orka-agents.github.io/orka/demo/). It runs the real dashboard in your
browser on sample data, with a guided tour, and sends nothing anywhere. The
[demo videos](https://orka-agents.github.io/orka/docs/demos) show the same features on a live
cluster.

See you in [Part 1](01-meet-orka.md).
