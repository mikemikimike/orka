---
title: "Part 4 — Running Orka for real: visibility, cost, and how it works inside"
slug: running-orka-for-real
date: 2026-10-21T13:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "What Orka records, how to trace and measure cost per merged pull request, and how it works inside."
---

*Part 4 of 4 in "Orka, piece by piece." [Start with the series introduction](00-series-intro.md).*

The first three parts were about what agents can do. This last part is for the person who
keeps the lights on: the platform engineer, the SRE, or the developer who ended up "owning the
AI thing." If agents are coworkers, this is the person who runs the office. Their questions are
different:

<!-- truncate -->

- What did the agents do, and can I prove it?
- Where do I see that in the tools I already use?
- How much is this costing, and is it worth it?
- What is actually running in my cluster, and what happens when something restarts?

Here is how Orka answers each one.

## Everything leaves a record

Every Task in Orka leaves a durable record you can come back to later:

- its **phase** (`Pending`, `Running`, `Succeeded`, `Failed`, ...) and the reason when it fails,
- its **result** and any **artifacts** it produced,
- and an **event timeline**: an ordered history of what happened during the Task — model calls,
  tool calls, approvals, delegations — not just the final answer.

For coding agents that change a repository, there is one more piece: a **delivery receipt**.
Remember the publisher from [Part 2](02-trust-by-design.md)? After it pushes, it checks the
remote branch and records exactly what landed. So "the agent said it opened a PR" becomes "Orka
verified this exact change is on this branch."

Worker logs are there for troubleshooting too, but treat the status, result, artifacts, and
timeline as the record; Orka does not promise to keep a complete raw log for every Task.

All of this is reachable from every door you met in [Part 1](01-meet-orka.md): the REST API,
`orka task status` and `orka task result` in the CLI, and the dashboard.

A useful rule from the docs: treat a Task as done only when its phase is `Succeeded`, and for a
coding agent that writes to a repository, check the delivery record too.

## See agents in the observability tools you already run

Orka speaks [OpenTelemetry](https://opentelemetry.io/), the open standard most observability
tools understand. Turn on telemetry and point Orka at your OpenTelemetry collector (the service
that gathers this data), and you get:

- **traces** that follow one request step by step through Orka, chat, tool calls, and the AI
  worker,
- **AI-specific metrics** such as how long model and tool calls take and how many tokens they
  use,
- alongside the usual Prometheus metrics and structured logs.

Telemetry is off by default. When it is on, Orka records metadata, token counts, and timings,
but not the text of prompts or completions; there is currently no setting that captures them.
Your traces show what happened and how long it took without quietly copying sensitive text
into another system. See
[Observability](https://orka-agents.github.io/orka/docs/observability).

## Measure cost in a way that means something

"We spent 40 million tokens this month" is a hard number to act on. Was that a lot? What did
we get for it?

Orka's [usage report](https://orka-agents.github.io/orka/docs/usage-and-pr-outcomes) frames
cost around outcomes. Its main measure is **tokens per merged pull request**, per team (a team
is a Kubernetes namespace). It follows each issue from planning through implementation, retries,
delegated helpers, reviews, and repairs, and it **includes failed and cancelled work** — because
you paid for that too. You can read it from the CLI (`orka usage summary -n orka-system`) or on the
dashboard's **Usage** page. It is honest about its limits: it
measures model usage, not developer productivity or hours saved, and it shows you where its
measurements have gaps.

## The dashboard

Orka's [web dashboard](https://orka-agents.github.io/orka/docs/ui) is built into Orka itself,
so there is nothing extra to deploy. It is off by default, and one Helm setting turns it on.
From there you can watch Tasks run, read results and timelines, approve or decline actions,
follow repository monitors and security findings, check usage, and chat with the
orchestrator. If you want to look before installing anything, the
[dashboard demo](https://orka-agents.github.io/orka/demo/) runs the real dashboard in your
browser on sample data.

## When a model provider has a bad day

Model services have outages and rate limits like any other service. For Orka's own `type: ai`
Agents, you can list backup Providers under `model.fallbacks`, for example "Anthropic first,
then OpenAI." Orka retries the main Provider first. If it keeps failing, rejects the key, or cannot be
reached, Orka moves to the next one on the list. Each backup is just another Provider, with its
own key in its own Secret.

## One address for every team

In Part 1, developers pointed Claude Code or Codex at Orka's compatible endpoints. In a bigger
organization, each team usually runs its own Orka in its own namespace, and nobody wants to
hand out a different address per team.

An optional [shared endpoint](https://orka-agents.github.io/orka/docs/operations/shared-compatibility-api)
fixes that. Everyone uses the same OpenAI- and Anthropic-compatible address; the caller's token
says which team they belong to, and the request goes to that team's Orka, with that team's
models and permissions. The shared endpoint itself holds no model keys and keeps no
conversations.

## How it works inside

You do not need to know this to use Orka. But if you operate it, a simple mental model helps
when something looks wrong.

### One controller, three places to run work

Orka is **one controller**: a single program running in your cluster that watches for Tasks and
turns them into work. Requests come in through the
API, CLI, dashboard, chat, compatible endpoints, gateways, and GitHub webhooks. The controller
checks who is asking, creates or updates the Task, and sends it to the right place:

| Task type | Where it runs |
| --- | --- |
| `container` | Its own short-lived Kubernetes Job |
| `ai` | Its own Kubernetes Job running Orka's AI worker |
| `agent` (built-in coding agents) | A private session inside a warm **RuntimePool**, or a dedicated one for an execution workspace |
| `agent` (an agent you plugged in, like AgentKit) | The service you registered, which you run yourself |

### Warm pools that scale to zero

Starting a coding agent from scratch is slow: big image, lots of setup. So instead of a fresh
Job per Task, Orka keeps warm **RuntimePools** for the built-in coding agents, kept separate
per namespace and per agent setup. A pool Pod hosts
several private sessions when it is busy, so a new Task usually starts in seconds, and it
**scales to zero** once it has been empty for a while. For bursty work, where agents spend
long stretches waiting on people and checks, that can save a good deal of compute.

When a pool needs to be replaced or scaled down — say, after an upgrade — Orka first lets it
finish the work it has (it "drains") before removing it. Running Tasks are not cut off halfway.

### Pinned images

Coding-agent pools run their images by **digest** (an exact fingerprint), not by a tag like
`latest`, so a running session can never have its image swapped underneath it. If you
configure a tag, Orka turns it into a digest when the controller starts, and a later restart
looks the tag up again. To be sure the agent that runs tomorrow is exactly the one you tested
today, configure the digest itself.

### Restarts are boring, on purpose

Controllers restart: upgrades, node maintenance, crashes. For coding agents, Orka records each
prompt attempt before sending it, and after a restart it never automatically resends one the
agent might already have received. If Orka truly cannot tell whether something finished, it
reports the outcome as *uncertain* rather than guessing. (`ai` and `container` Tasks run as
Kubernetes Jobs and follow their own retry rules.)

### Where state lives

Orka needs **no external database.** It keeps:

- **control decisions** — who is in charge, which session owns what, which branch is claimed —
  in Kubernetes' own records, including locks called Leases, where the cluster itself prevents
  two writers from stepping on each other,
- **records** — results, artifacts, transcripts, plans, memory, monitor history, usage — in an
  embedded SQLite database on the controller's volume.

Back them up together. The [upgrading guide](https://orka-agents.github.io/orka/docs/upgrading)
explains how.

## Day-two checklist

A short list for whoever takes over an Orka install:

- **Install:** one Helm command; see
  [Install Orka](https://orka-agents.github.io/orka/docs/installation).
- **Namespaces:** create resources in the namespace the controller watches. Things created
  elsewhere are silently ignored — the single most common "why isn't it running?"
- **Model access for coding agents:** set up the
  [provider proxy](https://orka-agents.github.io/orka/docs/provider-proxy) once.
- **Turn on what you need:** dashboard, telemetry, connectors, execution workspaces, and
  toolboxes are all off by default.
- **Upgrade carefully:** read the release notes and the
  [upgrading guide](https://orka-agents.github.io/orka/docs/upgrading), and back up
  first.
- **When something is stuck:** the
  [troubleshooting guide](https://orka-agents.github.io/orka/docs/troubleshooting)
  lists error messages, causes, and fixes; the
  [runbook](https://orka-agents.github.io/orka/docs/operations/runbook) covers what to do when
  things look stuck.

## Wrapping up the series

Over four parts we went from three small building blocks to agents that review every pull
request, turn issues into pull requests, work in teams, answer in Teams and Telegram, and
report their cost per merged PR. Two ideas carried us the whole way:

**Orka is composable.** Three building blocks — Provider, Agent, Task — and small additions
on top: a schedule, a monitor, a coordination block, a gateway, a sandbox, a tool. You add the
pieces you need and leave the rest off. Nothing new replaces what you already built. Orka
does not ask you for new infrastructure either: it runs on the Kubernetes cluster you have,
and plugs into the sandbox projects you choose.

**Orka meets people where they are.** `kubectl` for the Kubernetes crowd, a CLI for scripters,
a dashboard for clickers, chat for anyone, Claude Code and Codex for developers, GitHub for
maintainers, Teams and Telegram for everyone else. Almost every door leads to the same Task,
with the same keys kept safe, the same rules, and the same record; the coding-tool endpoints
share the same keys and model rules without needing a Task.

Underneath both ideas sits one habit from Part 2: keep jobs apart. For a coding agent, the part
that talks to the model, the part that does the work, and the part that delivers it are separate,
and the real model key and Git credential stay out of the agent's reach, so an agent that gets
fooled cannot take them.

## Where this is going

We started the series with a guess about where agents are heading: out of the chat window and
into how teams work, many kinds of them, acting for people, and waiting a lot in between. Orka
is built around that guess.

The agents will keep changing. There will be new coding tools, new models every few months,
agents from cloud platforms, and agents your own team builds for one job. Each of them should
be able to step into the same workplace: the same kind of Task, the same keys kept out of reach,
the same approvals, the same record. Swapping the agent should feel like swapping a part, not
rebuilding the shop. That is what all the small, composable pieces are for.

Orka is experimental and changing quickly, and the project wants to hear from you. Try it on a
laptop, break it, and tell us what you find.

**Get started:** [Getting started](https://orka-agents.github.io/orka/docs/getting-started) ·
[Dashboard demo](https://orka-agents.github.io/orka/demo/) ·
[Demo videos](https://orka-agents.github.io/orka/docs/demos) ·
[GitHub](https://github.com/orka-agents/orka) ·
[Open an issue](https://github.com/orka-agents/orka/issues)
