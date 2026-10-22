---
title: "Part 3 — Agents that keep working: automation, teamwork, and your own tools"
slug: agents-that-keep-working
date: 2026-10-21T12:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "Schedules, pull request review, issue-to-PR automation, agent teams, memory, chat gateways, and your own tools, built from the same three pieces."
---

*Part 3 of 4 in "Orka, piece by piece." [Start with the series introduction](00-series-intro.md).*

So far, a person has started every Task. That is useful, but it is not where the value of
agents really shows up. The value shows up when agents become part of how your team already
works: reviewing every pull request, picking up issues, running the nightly checks nobody
wants to run, answering questions in the team chat.

<!-- truncate -->

This part shows how to get there. The good news: **you do not learn a new system for each of
these.** Every feature here is built from the same pieces from [Part 1](01-meet-orka.md) —
Provider, Agent, Task — plus one small addition. And everything from
[Part 2](02-trust-by-design.md) still applies: the same key handling, the same publisher, the
same approvals, the same records.

Here is the map of this part:

| You want agents to... | Add this | Built on |
| --- | --- | --- |
| Run on a timer | `schedule` on a Task | Task |
| Care for a repository | `RepositoryMonitor` | Agents + Tasks |
| Work as a team | `coordination` on an Agent | Agent + child Tasks |
| Remember things | Memory proposals | Agent + review |
| Pick up where they left off | Sessions, session import, and execution workspaces | Task |
| Answer in Teams, Telegram, or to other agents | A gateway adapter | Agent + Tasks |
| Know your team's ways and use your tools | Skills, a persona, tools, and toolboxes | Agent |
| Be an agent you built yourself | An AgentKit image | Agent |

## Agents on a schedule

Any Task can run on a schedule. Add one line:

```yaml
apiVersion: core.orka.ai/v1alpha1
kind: Task
metadata:
  name: nightly-triage
  namespace: orka-system
spec:
  type: ai
  agentRef:
    name: triage-agent
  prompt: "Summarize new issues opened in the last 24 hours"
  schedule: "0 2 * * *"        # every day at 02:00
  timeZone: "America/New_York"
```

The Task now acts as a template. Every night Orka creates a fresh copy and runs it, so
`kubectl get tasks` shows you the history at a glance. If you have used a Kubernetes CronJob,
this will feel familiar, with sensible defaults: overlapping runs are skipped, and if Orka
was down when a run was due, it does not replay a backlog of missed runs when it comes back.

Because a scheduled Task is just a Task, it can be any of the three types: a model summarizing
issues, a coding agent checking dependencies, or a container running a report.

## Agents that care for a repository

Here is where Orka starts to feel like a teammate.

### Review every pull request

A [repository monitor](https://orka-agents.github.io/orka/docs/repository-monitors) puts a
GitHub repository "under agent care." When a pull request arrives, it starts a review Task
bound to the exact commit, and can post the findings back as a review comment.

Notice how Part 2 shows up here. The review Task is read-only and holds no Git credential.
The comment is posted by Orka itself, after it re-checks that the pull request has not changed,
and every GitHub write goes into an audit log. The monitor never merges a pull request; it can
publish a "ready" check that GitHub's own auto-merge waits on, and your normal branch rules stay
in charge.

### Turn an issue into a pull request

Label an issue `orka:implement` and the monitor takes it from there:

1. A planning agent reads the issue and writes a plan. If the plan says "blocked" or "needs a
   human," it stops and tells you why.
2. A coding agent implements the plan on a branch Orka chose.
3. The publisher checks and pushes the change, and Orka opens the pull request.
4. A reviewer agent reviews it. If review or CI asks for changes, a repair agent fixes them.
5. This repeats, within limits, until the pull request is ready or clearly stuck.

Each role — planner, implementer, reviewer, repairer — is just an Agent you configured. Want
Claude Code to implement and Codex to review? Point the roles at different Agents. That is
composability at work.

Before acting on a label, Orka checks the webhook signature and that the person who added the
label really has permission on the repository. Labels are a convenient trigger, not an open
door. To stop Orka from taking further steps on a particular issue or pull request, add the
`orka:pause` label. A Task that is already running may still finish and publish, so cancel it
if you need it stopped right away. Removing the label lets Orka look at the issue again and
decide what to do next. [Issue-to-PR automation](https://orka-agents.github.io/orka/docs/guides/issue-to-pr-automation)
has a complete runnable example.

### Find security issues

[Repository security scanning](https://orka-agents.github.io/orka/docs/repository-security-scanning)
builds an editable threat model of a repository and scans its history and new commits for
likely vulnerabilities. It can double-check selected findings by trying them in an isolated
worker (by default only a few per run), and it can propose a fix as a pull request. A person
always decides when to create that fix.

## Agents that work as a team

Some jobs are too big for one agent, or need different skills: a backend change, a frontend
change, and a review. With
[multi-agent coordination](https://orka-agents.github.io/orka/docs/multi-agent-coordination),
one **coordinator** agent splits the work and hands pieces to specialists. The specialists are
ordinary Agents like the ones from Part 1; here, `backend-dev` is a plain AI Agent and `coder`
is a Codex Agent.

```yaml
apiVersion: core.orka.ai/v1alpha1
kind: Agent
metadata:
  name: coordinator
  namespace: orka-system
spec:
  providerRef:
    name: anthropic
  coordination:
    enabled: true
    maxDepth: 2
    maxConcurrentChildren: 3
    allowedAgents:
      - name: backend-dev
      - name: coder
```

The division of labor is clear:

- **The model decides** what to delegate and how to combine the results.
- **Orka decides** what is allowed: which Agents may be used, how deep delegation may go, and
  how many helpers run at once.

Each delegation is just a child Task. You can watch it, cancel it, and audit it like any other.
The specialists can be a mix of plain AI agents and coding agents: a coordinator can ask a
Claude-based researcher for context, then hand the code change to Codex.

### Autonomous Tasks

Add `autonomous: true` and a round limit to the coordinator's `coordination` block, and it
works in rounds toward a goal:

```yaml
  coordination:
    enabled: true
    autonomous: true
    maxIterations: 20   # leave this out and the rounds are unlimited
```

Each round it reads its saved plan, delegates, updates the plan, and stops when the goal is
met or it reaches `maxIterations`. Each round runs as its own Kubernetes Job (a one-off run of a
container) and the plan is
saved between rounds. A failed round can still fail the whole Task unless you add a retry
policy. Autonomous mode is for Orka's own `type: ai` coordinators; the built-in coding agents
do not support it. Combine this with human approval from Part 2 and the coordinator
pauses whenever it needs a person's OK. See
[Autonomous Tasks](https://orka-agents.github.io/orka/docs/autonomous-tasks).

## Agents that remember — carefully

It is helpful when an agent remembers "this project uses `make test`, not `go test`" or "we
release on Tuesdays." It is dangerous if *any* model output can quietly rewrite what every
future agent believes.

Orka's [memory](https://orka-agents.github.io/orka/docs/memory) takes the careful route:

1. An agent **proposes** a memory, using a `remember` tool Orka gives it.
2. A person **reviews** it through the API.
3. Accepted memories are **applied** in a separate, explicit step.
4. Only then do future Tasks in that namespace see it.

Every step records who did it. Agents can also search past conversation transcripts when they
need more context.

## Agents that pick up where they left off

Real work rarely fits in one go. You ask an agent to start a refactor, look at what it did, and
come back an hour later with "now update the tests." Sometimes you even start on your laptop and
want to hand the work over to the cluster. For any of that, the agent needs two things: the
conversation so far, and the files it was working on.

An agent gets more useful the longer it can build on earlier work, but every kind of
remembering also needs someone in charge of it. Orka keeps three kinds apart:

| What is kept | Feature | Lasts for | Who controls it |
| --- | --- | --- | --- |
| The conversation | Session (below) | Across Tasks, until it is deleted or cleaned up | Whoever sends the Tasks |
| The files | Execution workspace (below) | Until it is deleted or expires under rules set by whoever runs Orka | Whoever runs Orka |
| Facts worth keeping | Memory (above) | Until someone removes them | A person approves each one |

### Sessions: the conversation

A **Session** is a conversation that spans several Tasks. Give each Task the same `sessionRef`
and Orka hands the agent the earlier conversation, so your follow-up makes sense:

```yaml
spec:
  sessionRef:
    name: refactor-auth
    create: true   # start the Session if it does not exist yet
```

Tasks in one Session run one at a time, so two messages never trip over each other. If a
follow-up depends on the previous answer, wait for that Task to finish before sending it.

### Start on your laptop, continue in Orka

Say you have been working with Codex on your laptop, and the job turns out to be bigger than
you thought. You would like it to keep going in the cluster, with the same conversation, while
you close your laptop.

Orka can import that conversation into a new Session. First point the `orka` CLI at your Orka
server and sign in (`orka config set-server` and `orka config set-token`), and stop Codex. Then:

```bash
orka session migrate import refactor-auth-laptop \
  --codex-home ~/.codex \
  --thread <your Codex thread ID> \
  --journal-dir ~/orka-import-journal \
  --source-stopped
```

The Session name must be new; import never overwrites an existing Session. Now send a normal
Codex Task with `sessionRef.name: refactor-auth-laptop` and `create: true`. The agent in the
cluster picks up the *same* Codex conversation, not a summary of it. After each successful Task,
Orka saves Codex's own record of the conversation, so the next Task continues where that one
stopped. If a Task fails, the last good record is kept.

When you want it back, `orka session migrate export refactor-auth-laptop`, pointed at a fresh Codex
folder on your laptop, installs the latest saved conversation there, and you resume it locally.

A few honest limits today: this works for Codex only, with a specific Codex version, and only
for conversations that never used Codex's reasoning mode (reasoning leaves encrypted data in the
history, and turning it off later does not remove that). Bring a conversation back into
a fresh Codex folder rather than your everyday one. The
[session migration guide](https://orka-agents.github.io/orka/docs/guides/codex-session-migration)
has the details.

### Execution workspaces: the files, and a place of their own

By default, a coding agent runs in the shared pool from Part 1. That pool is fast, but it is
shared, and it shuts down when nobody is using it. An **execution workspace** gives one
conversation a sandbox of its own instead.

Orka does not build that sandbox itself. It plugs into sandbox projects you run alongside it.
Two are supported today:

- **[Agent Sandbox](https://github.com/kubernetes-sigs/agent-sandbox)**, a Kubernetes community
  project that runs each agent in its own sandbox Pod with its own storage.
- **[Agent Substrate](https://github.com/agent-substrate/substrate)**, which runs each agent under
  gVisor (a sandbox that sits between a program and the machine) on dedicated machines, and can
  take snapshots of an agent's files.

Here is what that gives Orka:

| With a workspace, an agent can... | Agent Sandbox | Agent Substrate |
| --- | --- | --- |
| Work behind its own wall, one conversation per sandbox | Yes | Yes |
| Suspend when idle, and resume later with its files intact | Yes | Yes |
| Save a checkpoint of its files that outlives the workspace | No | Yes |
| Restore a checkpoint into a new workspace, or fork it into several | No | Yes |

### Suspend and resume

Remember from Part 1 that agent work often comes in bursts, with waiting in between. A shared
pool shuts down once it has been empty for a while; a workspace goes one step further. With a workspace that allows it, Orka **suspends** the agent when a Task finishes: it stops the agent completely and
keeps only its files. Agent Sandbox keeps the files on a storage volume; Agent Substrate takes a snapshot of
them. Orka double-checks that the old agent is really gone before it calls the workspace
suspended, so nothing is quietly left running.

When your next message arrives, Orka **resumes**: it starts a fresh agent on the saved files and
continues the Session. Two things are deliberately *not* brought back:

- **Running memory.** The agent starts fresh, like a computer after a restart. Its files and the
  conversation are there; whatever was only in memory is not.
- **Old credentials.** The resumed agent gets new ones, so nothing that should have expired
  comes back to life.

Whoever runs Orka writes those rules into a *workspace class*, a named set of workspace
settings (more on choosing one below). For example: "suspend between Tasks, throw away saved
files nobody has used for a day, and never keep a workspace longer than a week."

### Snapshots, checkpoints, and restore

Suspending is about one workspace taking a nap. A **checkpoint** is a saved copy of a
workspace's files that you keep on purpose, and it lives on even after the original workspace
is deleted. Agent Substrate supports them.

From a checkpoint you can:

- **Restore:** start a brand-new workspace from it. Use this to undo a wrong turn, or to recover
  after the original is gone.
- **Fork:** start *several* new workspaces from the same checkpoint. Each one is independent, so
  you can let one agent try approach A and another try approach B from the exact same starting
  point.

The same rules as resume apply: a restored workspace gets the files, never the old running
memory or old credentials. Checkpoints are also protected like everything else in Orka: you need
permission on the source workspace to save one, and permission on the checkpoint to use it.

### Choosing a workspace

Choosing a workspace is the same composable idea again. Whoever runs Orka publishes a few named
*workspace classes*, such as "Agent Sandbox, suspend between Tasks." A Task just asks for one
by name:

```yaml
spec:
  type: agent
  agentRef:
    name: codex-agent
  sessionRef:
    name: refactor-auth
    create: true
  execution:
    workspace:
      classRef:
        name: sandbox-coding
      reusePolicy: session   # keep using the same workspace for this conversation
```

The Task does not say which sandbox project is underneath, and it does not need to. Everything
else from Part 2 still applies: the agent still has no Git credential, and the publisher still
does the pushing.

Workspaces are off by default, and each sandbox project is installed and run separately from
Orka. To see them in action, watch
[suspending a workspace and keeping its files](https://www.youtube.com/watch?v=DyS9JioSRa0) (Agent
Sandbox) and [checkpointing and restoring a workspace](https://www.youtube.com/watch?v=jsdRB-0LLAc)
(Agent Substrate). The [execution workspaces guide](https://orka-agents.github.io/orka/docs/concepts/execution-workspaces)
has the setup.

## Agents where your team already talks

[Gateways](https://orka-agents.github.io/orka/docs/gateways) bring agents into the
conversations people already have:

- **Microsoft Teams** — personal chats and opt-in shared rooms
- **Telegram** — private chats
- **A2A** — any other agent or system that speaks the
  [Agent2Agent protocol](https://a2a-protocol.org/) can call an Orka agent

A message arrives, Orka runs the matching Agent as an ordinary Task, and the reply goes back to
the same conversation. Follow-up messages continue the same session. Orka stores every message
before acknowledging it, so nothing is lost if something restarts, and a message from an unknown
conversation or sender is refused rather than guessed at.

Each gateway is a small adapter you deploy next to Orka. It holds the platform's credentials;
Orka holds everything else. If your team uses something else, the protocol is documented so you
can write your own adapter.

Add this to the other doors from Part 1 — `kubectl`, the CLI, the dashboard, chat, and the
OpenAI- and Anthropic-compatible endpoints — and the same Agent can be reached from almost
anywhere your team works.

## Agents with your tools

The last way to grow an agent is to give it more to work with. There are four options, from
smallest to biggest.

### Teach it how your team works: skills and a persona

A **skill** is a short, reusable set of instructions, such as "run `make lint` after every
change" or "write release notes in this format." You save it in Orka once
(`orka skill import ./SKILL.md --name release-notes -n orka-system`) and list it by name on any of Orka's own
`type: ai` Agents that need it. Keep skills short: their text is sent with every request to
the model, so each one costs a little on every call. Coding agents do not take skills; put that
guidance in their instructions instead.

A **soul** sets an Agent's personality and tone, kept apart from its job description, such as
"be direct, and say what you could not check." It works for `type: ai` Agents and the built-in
coding agents alike. It shapes how the agent talks; it does not grant or remove any permission.

### Connect your APIs

A *tool* is something an agent can call: a web search, a calculator, your company's inventory
API. You can give Orka's own `type: ai` agents tools that call your HTTP APIs. You can also
connect them to an MCP server your team already runs. MCP, the Model Context Protocol, is a
common way to offer tools to AI agents. The credentials for these tools stay behind the
outbound access policies from Part 2, never with the agent. See
[Agents, skills, and tools](https://orka-agents.github.io/orka/docs/guides/agents-skills-and-tools).

### Give coding agents more programs: toolboxes

A coding agent can only run the programs that come with it. Ask Codex to edit a YAML file with
`yq`, and if `yq` is not there, it is stuck.

A [toolbox](https://orka-agents.github.io/orka/docs/guides/toolboxes) fixes that. It is an
ordinary container image that holds extra programs in one folder. You list it on the Agent, and
Orka makes those programs available to the agent. Here is the `codex-agent` from Part 1 with a
toolbox added:

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
    toolboxes:
      - image: registry.example.com/tools/yq-jq@sha256:<digest>
        mountPath: /opt/yq-jq   # the folder inside the image that holds the programs
        pathEntries: [bin]      # lets the agent run them by name
```

A toolbox adds programs and nothing else. It brings no passwords, no extra network access, and
no admin rights. The image is named by its exact fingerprint (`sha256:...`) so it cannot change
under you. Toolboxes are off by default; whoever runs Orka turns them on and lists the image
registries they trust.

### Bring an agent you built: AgentKit

The coding agents so far are well-known tools: Codex, Claude Code, Copilot CLI, OpenCode.
Sometimes you want your *own* agent, with your own instructions and tools, built for one job.

[AgentKit](https://github.com/orka-agents/agentkit), a companion project, lets you describe that
agent in one YAML file and build it into an ordinary container image:

```yaml
#syntax=ghcr.io/orka-agents/agentkit/agentkit:latest
apiVersion: v1alpha1
kind: Agent
metadata:
  name: url-summarizer
model:
  provider: openai-compatible
  baseURL: https://api.openai.com/v1
  name: gpt-4o-mini
  apiKeyEnv: OPENAI_API_KEY   # only the name of the variable; the key is never in the image
instructions: |
  Summarize any URL the user gives you in three bullet points.
expose:
  openai: true
```

```bash
docker buildx build . -f agentkitfile.yaml -t url-summarizer:latest
```

That image runs anywhere containers run, including your laptop. To use it from Orka, whoever
runs Orka does a one-time setup: add Orka's small supervisor program on top of the image (the
supervisor runs your agent and keeps credentials and permissions outside it), deploy it, and
register it as an `AgentRuntime`. Orka checks that it behaves correctly before sending it any
work. The [bring your own runtime](https://orka-agents.github.io/orka/docs/guides/bring-your-own-agent-runtime)
guide walks through this. After that, an Agent just points at it:

```yaml
apiVersion: core.orka.ai/v1alpha1
kind: Agent
metadata:
  name: summarizer
  namespace: orka-system
spec:
  runtime:
    runtimeRef:
      name: url-summarizer
```

This Agent sets no model or instructions, because those are already in the image. Tasks use it
like any other Agent, with the same records. If the runtime is set up for it, human approval
from Part 2 can cover the tools Orka runs on its behalf. Agents hosted in Microsoft Foundry plug in the same way, through the
[Foundry runtime](https://github.com/orka-agents/orka-runtime-foundry).

## What you have now

Look back at what we added in this part: a `schedule` field, a `RepositoryMonitor`, a
`coordination` block, a review step for memory, a Session and a workspace, a gateway adapter,
skills, tools, a toolbox, and an agent you built yourself. Each one is
small. None of them replaced anything from Part 1 or Part 2. They snap onto the same
Provider–Agent–Task core, and they inherit the same security and the same records.

That is what "composable" means in practice. You start with one Task, and you grow toward
"agents review every PR, implement labeled issues, and answer in Teams" one piece at a time,
without rebuilding anything.

## Next up

In **[Part 4 — Running Orka for real](04-running-orka-for-real.md)**, we look at it from the operator's side: what Orka
records, how to trace agents with OpenTelemetry, how to measure cost as tokens per merged pull
request, and how Orka is built underneath.

**Read more:** [Scheduled tasks](https://orka-agents.github.io/orka/docs/guides/scheduled-tasks) ·
[Repository monitors](https://orka-agents.github.io/orka/docs/repository-monitors) ·
[Multi-agent coordination](https://orka-agents.github.io/orka/docs/multi-agent-coordination) ·
[Gateways](https://orka-agents.github.io/orka/docs/gateways)
