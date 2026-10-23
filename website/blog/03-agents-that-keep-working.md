---
title: "Part 3 — Agents that keep working: schedules, conversations, teamwork, and your own tools"
slug: agents-that-keep-working
date: 2026-10-21T12:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "Agents on a schedule, in your chat apps, working as a team, asking before they act, and using your own tools, plus what software teams get on top."
---

*Part 3 of 4 in "Orka, piece by piece." [Start with the series introduction](00-series-intro.md).*

So far, a person has started every Task. That is useful, but it is not where the value of
agents really shows up. In the introduction we said agents are moving out of the chat window
and into how people work: drafting the week's posts before anyone asks, answering a text at
night, picking up the issue nobody had time for. The difference is a bit like the difference
between a tool you pick up and a teammate who shows up. This part is about that move.

<!-- truncate -->

We will follow the three agents from the introduction:

- a **marketing assistant** for a small team, which drafts posts from a product brief, checks
  last week's numbers, and asks before it publishes anything,
- a **personal assistant** you text from your phone, which keeps track of the conversation and,
  through tools you connect, works with your calendar and your notes,
- and a **coding agent**, which reviews and changes code.

The good news: **you do not learn a new system for each of these.** Every feature here is the
same pieces from [Part 1](01-meet-orka.md) — Provider, Agent, Task — plus one small addition.
And everything from [Part 2](02-trust-by-design.md) still applies: the same key handling, the
same approvals, the same records.

| You want agents to... | Add this |
| --- | --- |
| Run on a timer | `schedule` on a Task |
| Answer where people already talk | A gateway adapter |
| Remember the conversation, and what matters | Sessions and memory |
| Work as a team, toward a goal, and ask first | `coordination` on an Agent |
| Know your ways and use your tools | Skills, a persona, and tools |
| Be an agent you built yourself | AgentKit or another plugged-in runtime |
| Care for a repository | Repository monitors, workspaces, and toolboxes (for software teams) |

## Agents on a schedule

Any Task can run on a schedule. Here is the marketing assistant's Monday morning job:

```yaml
spec:
  type: ai
  agentRef:
    name: marketing-assistant
  prompt: "Draft next week's posts from the product brief and last week's numbers."
  schedule: "0 8 * * 1"        # every Monday at 08:00
  timeZone: "America/New_York"
```

The Task now acts as a template. Every Monday Orka creates a fresh copy and runs it, so
`kubectl get tasks` shows you the history at a glance. If you have used a Kubernetes CronJob,
this will feel familiar, with sensible defaults: overlapping runs are skipped, and if Orka
was down when a run was due, it does not replay a backlog of missed runs when it comes back.

Because a scheduled Task is just a Task, it can be any of the three types: a model drafting
posts, a coding agent checking dependencies overnight, or a container running a report.

## Agents where people already talk

Plenty of people will never open a dashboard to talk to an agent. They will text it.
[Gateways](https://orka-agents.github.io/orka/docs/gateways) bring agents into the
conversations people already have:

- **Microsoft Teams**: personal chats and opt-in shared rooms, so the marketing team can ask
  the assistant for a draft right where they plan the week
- **Telegram**: private chats
- **iMessage**: texts from your phone, through
  [an adapter](https://github.com/orka-agents/orka-gateway-imessage) that runs on a Mac signed
  in to Messages, ideally with an account of its own, and reachable from Orka over a public
  secure address
- **A2A**: any other agent or system that speaks the
  [Agent2Agent protocol](https://a2a-protocol.org/) can call an Orka agent

A message arrives, Orka runs the matching Agent as an ordinary Task, and the reply goes back to
the same conversation. Orka stores every message before acknowledging it, so nothing is lost if
something restarts, and a message from an unknown conversation or sender is refused rather than
guessed at.

The personal assistant shows why that care matters. A Messages account is usually *personal*,
so the iMessage adapter is deliberately narrow. It forwards messages only from people on an
allowlist. It replies only in a direct chat where someone on that list wrote first, and never
in a group chat, where a reply would reach everyone. It never replays old history, never sends
the same reply twice, and handles text only.

Each gateway is a small adapter you deploy next to Orka. It holds the platform's credentials;
Orka holds everything else. If your team lives somewhere else, the protocol is documented so you
can write your own adapter.

## Agents that remember, carefully

An agent gets more useful the longer it can build on earlier work. But every kind of
remembering needs someone in charge of it, so Orka keeps the kinds apart.

### The conversation: Sessions

A **Session** is a conversation that spans several Tasks. When you text the personal assistant
"move my 3pm to tomorrow" and later "actually, make it Friday," the second message only makes
sense with the first. Gateways handle this for you: follow-up messages in the same chat
continue the same Session. Tasks you send yourself can join a Session by name, too.

Tasks in one Session run one at a time, so two messages never trip over each other. If a
follow-up depends on the previous answer, wait for that Task to finish before sending it.

### What matters: Memory

It is helpful when an agent remembers "our brand never uses exclamation marks" or "I prefer
morning meetings." It is dangerous if *any* model output can quietly rewrite what every future
agent believes. Orka's [memory](https://orka-agents.github.io/orka/docs/memory) takes the
careful route:

1. An agent **proposes** a memory, using a `remember` tool Orka gives it.
2. A person **reviews** it through the API.
3. Accepted memories are **applied** in a separate, explicit step.
4. Only then do future Tasks in that namespace see it.

Every step records who did it. Agents can also search past conversation transcripts when they
need more context.

| What is kept | Feature | Lasts for | Who controls it |
| --- | --- | --- | --- |
| The conversation | Session | Across Tasks, until it is deleted or cleaned up | Whoever sends the Tasks |
| Facts worth keeping | Memory | Until someone removes them | A person approves each one |
| A coding agent's files | Execution workspace (see [For software teams](#for-software-teams)) | Until it is deleted or expires under rules set by whoever runs Orka | Whoever runs Orka |

## Agents that work as a team

Some jobs are too big for one agent, or need different skills. A product launch needs research
on what competitors are saying, a set of drafts, and a final pass. With
[multi-agent coordination](https://orka-agents.github.io/orka/docs/multi-agent-coordination),
one **coordinator** agent splits the work and hands pieces to specialists, which are ordinary
Agents like the ones from Part 1: a `researcher` with web search, a `writer` that knows the
brand.

The division of labor is clear:

- **The model decides** what to delegate and how to combine the results.
- **Orka decides** what is allowed: which Agents may be used, how deep delegation may go, and
  how many helpers run at once.

Each delegation is just a child Task. You can watch it, cancel it, and audit it like any other.
The specialists can be a mix of plain AI agents and coding agents: on a software team, the same
coordinator could ask a researcher for context, then hand the code change to Codex.

### Working toward a goal, and asking first

Turn on **autonomous mode** and the coordinator works in rounds toward a goal. Each round it
reads its saved plan, delegates, and updates the plan. It stops when the goal is met or it hits
the round limit you set (without one, the rounds are unlimited). Each round runs as its own
Kubernetes Job (a one-off run of a container), and the plan is saved between rounds. A failed
round can still fail the whole Task unless you add
a retry policy.

This is also where the marketing assistant gets its most important habit: **ask before
publishing.** Mark the tool that publishes a post as needing approval, and the agent stops
right before it would run, with the exact post saved. A person reads it in the dashboard or
the CLI and approves or declines; on approval, Orka runs exactly what they saw. Here is the
whole setup, team, goal, and sign-off, on the coordinator:

```yaml
spec:
  coordination:
    enabled: true
    allowedAgents:            # who it may hand work to
      - name: researcher
      - name: writer
    maxConcurrentChildren: 3
    autonomous: true          # work in rounds toward the goal
    maxIterations: 10
    approvalRequiredTools:    # ask a person first
      - publish-post          # a Tool you defined that calls your social media scheduler
```

Autonomous mode is for Orka's own `type: ai` agents; the built-in coding agents do not support
it. See [Autonomous Tasks](https://orka-agents.github.io/orka/docs/autonomous-tasks).

## Agents with your tools

The next way to grow an agent is to give it more to work with.

### Teach it your ways: skills and a persona

A **skill** is a short, reusable set of instructions, such as "follow these brand guidelines"
or "write release notes in this format." You save it in Orka once and list it by name on any
of Orka's own `type: ai` Agents that need it. Keep skills short: their text is sent with every
request to the model, so each one costs a little on every call. Coding agents do not take
skills; put that guidance in their instructions instead.

A **soul** sets an Agent's personality and tone, kept apart from its job description, such as
"warm, brief, and never pushy." It works for `type: ai` Agents and the built-in coding agents
alike. It shapes how the agent talks; it does not grant or remove any permission.

### Connect your apps and APIs

A *tool* is something an agent can call: a web search, your analytics dashboard's API, your
social media scheduler, your calendar. You can give Orka's own `type: ai` agents tools that
call your HTTP APIs, or connect them to an MCP server your team already runs. MCP, the Model
Context Protocol, is a common way to offer tools to AI agents. The credentials for these tools
stay behind the outbound access policies from Part 2, never with the agent: the marketing
assistant can post through your scheduler without ever seeing its key. See
[Agents, skills, and tools](https://orka-agents.github.io/orka/docs/guides/agents-skills-and-tools).

### Bring an agent you built: AgentKit

Sometimes you want your *own* agent, with your own instructions and tools, built for one job:
a link summarizer, a support triager, a meeting-notes writer.

[AgentKit](https://github.com/orka-agents/agentkit), a companion project, lets you describe that
agent in one YAML file (its model, its instructions, its tools) and build it into an ordinary
container image with Docker. The image never contains the model key, only the name of the
setting the key will come from.

That image runs anywhere containers run, including your laptop. To use it from Orka, whoever
runs Orka does a one-time setup: add Orka's small supervisor program on top of the image (the
supervisor runs your agent and keeps credentials and permissions outside it), deploy it, and
register it as an `AgentRuntime`. Orka checks that it behaves correctly before sending it any
work. The [bring your own runtime](https://orka-agents.github.io/orka/docs/guides/bring-your-own-agent-runtime)
guide walks through this. After that, an Agent just points at it by name, with no model or
instructions of its own, because those are already in the image. Tasks use it like any other
Agent, with the same records. If the runtime is set up for it, human approval from Part 2 can
cover the tools Orka runs on its behalf. Agents hosted in Microsoft Foundry plug in the same
way, through the [Foundry runtime](https://github.com/orka-agents/orka-runtime-foundry).

## For software teams

Everything above works for any kind of agent. Coding is the use case Orka has gone deepest on,
though, and software teams get a few more pieces on top. If you do not work with code, you can
skip ahead to [what you have now](#what-you-have-now).

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
4. A reviewer agent reviews it. If review or the automated tests (CI) ask for changes, a repair agent
   fixes them.
5. This repeats, within limits, until the pull request is ready or clearly stuck.

Each role — planner, implementer, reviewer, repairer — is just an Agent you configured. Want
Claude Code to implement and Codex to review? Point the roles at different Agents.

Before acting on a label, Orka checks the webhook signature and that the person who added the
label really has permission on the repository. To stop Orka from taking further steps on a
particular issue or pull request, add the `orka:pause` label. A Task that is already running
may still finish and publish, so cancel it if you need it stopped right away. Removing the label
lets Orka look at the issue again and decide what to do next.
[Issue-to-PR automation](https://orka-agents.github.io/orka/docs/guides/issue-to-pr-automation)
has a complete runnable example.

### Find security issues

[Repository security scanning](https://orka-agents.github.io/orka/docs/repository-security-scanning)
builds an editable threat model of a repository and scans its history and new commits for
likely vulnerabilities. It can double-check selected findings by trying them in an isolated
worker (by default only a few per run), and it can propose a fix as a pull request. A person
always decides when to create that fix.

### Start on your laptop, continue in Orka

Say you have been working with Codex on your laptop, and the job turns out to be bigger than
you thought. You would like it to keep going in the cluster, with the same conversation, while
you close your laptop.

Orka can import that conversation. Stop Codex, and one `orka session migrate import` command
copies the conversation into a new Session in the cluster (import never overwrites an existing
one). Send a normal Codex Task to that Session, and the agent in the cluster picks up the
*same* Codex conversation, not a summary of it. After each successful Task, Orka saves Codex's
own record of the conversation, so the next Task continues where that one stopped. If a Task
fails, the last good record is kept. When you want it back, `orka session migrate export`
installs the latest saved conversation into a fresh Codex folder on your laptop.

A few honest limits today: this works for Codex only, with a specific Codex version, and only
for conversations that never used Codex's reasoning mode (reasoning leaves encrypted data in the
history, and turning it off later does not remove that). Bring a conversation back into a fresh
Codex folder rather than your everyday one. The
[session migration guide](https://orka-agents.github.io/orka/docs/guides/codex-session-migration)
has the details.

### A workspace of its own

By default, a coding agent runs in the shared pool from Part 1. That pool is fast, but it is
shared, and it shuts down when nobody is using it. An **execution workspace** gives one
conversation a sandbox of its own instead, with its files kept between Tasks.

Orka does not build that sandbox itself. It plugs into sandbox projects you run alongside it:

- **[Agent Sandbox](https://github.com/kubernetes-sigs/agent-sandbox)**, a Kubernetes community
  project that runs each agent in its own sandbox Pod with its own storage.
- **[Agent Substrate](https://github.com/agent-substrate/substrate)**, which runs each agent under
  gVisor (a sandbox that sits between a program and the machine) on dedicated machines, and can
  take snapshots of an agent's files.

| With a workspace, an agent can... | Agent Sandbox | Agent Substrate |
| --- | --- | --- |
| Work behind its own wall, one conversation per sandbox | Yes | Yes |
| Suspend when idle, and resume later with its files intact | Yes | Yes |
| Save a checkpoint of its files that outlives the workspace | No | Yes |
| Restore a checkpoint into a new workspace, or fork it into several | No | Yes |

**Suspend and resume.** Remember from Part 1 that agent work often comes in bursts, with waiting
in between. With a workspace that allows it, Orka **suspends** the agent when a Task finishes:
it stops the agent completely and keeps only its files. Agent Sandbox keeps the files on a
storage volume; Agent Substrate takes a snapshot of them. Orka double-checks that the old agent
is really gone before it calls the workspace suspended, so nothing is quietly left running.
When your next message arrives, Orka **resumes**: it starts a fresh agent on the saved files
and continues the Session. Two things are deliberately *not* brought back: whatever was only in
the agent's running memory (it starts fresh, like a computer after a restart), and its old
credentials (it gets new ones, so nothing that should have expired comes back to life).

**Checkpoints, restore, and fork.** A **checkpoint** is a saved copy of a workspace's files
that you keep on purpose, and it lives on even after the original workspace is deleted. Agent
Substrate supports them. From a checkpoint you can *restore* a brand-new workspace, to undo a
wrong turn or recover after the original is gone, or *fork* several independent workspaces, so
one agent can try approach A and another approach B from the exact same starting point. A
restored workspace gets the files, never the old running memory or old credentials, and you
need permission on the source workspace to save a checkpoint and permission on the checkpoint
to use it.

**Choosing one.** Whoever runs Orka publishes a few named *workspace classes*, such as "Agent
Sandbox, suspend between Tasks, throw away saved files nobody has used for a day, and never
keep a workspace longer than a week." A Task asks for one by name and does not need to know
which sandbox project is underneath. Everything from Part 2 still applies: the agent still has
no Git credential, and the publisher still does the pushing.

Workspaces are off by default, and each sandbox project is installed and run separately from
Orka. To see them in action, watch
[suspending a workspace and keeping its files](https://www.youtube.com/watch?v=DyS9JioSRa0) (Agent
Sandbox) and [checkpointing and restoring a workspace](https://www.youtube.com/watch?v=jsdRB-0LLAc)
(Agent Substrate). The [execution workspaces guide](https://orka-agents.github.io/orka/docs/concepts/execution-workspaces)
has the setup.

### More programs for coding agents: toolboxes

A coding agent can only run the programs that come with it. Ask Codex to edit a YAML file with
`yq`, and if `yq` is not there, it is stuck. A
[toolbox](https://orka-agents.github.io/orka/docs/guides/toolboxes) fixes that: an ordinary
container image that holds extra programs in one folder. You list it on the Agent, say which
folder holds the programs, and Orka makes them available to the agent by name.

A toolbox adds programs and nothing else. It brings no passwords, no extra network access, and
no admin rights. The image is named by its exact fingerprint (`sha256:...`) so it cannot change
under you. Toolboxes are off by default; whoever runs Orka turns them on and lists the image
registries they trust.

## What you have now

Look back at what we added in this part: a `schedule` field, a gateway adapter, a Session, a
review step for memory, a `coordination` block with an approval on one tool, skills, tools,
and an agent you built yourself. Software teams add a repository monitor, a workspace, and a
toolbox. Each one is small. None of them replaced anything from Part 1 or Part 2. They snap
onto the same Provider–Agent–Task core, and they inherit the same security and the same records.

That is what "composable" means in practice. A marketing assistant that drafts every Monday
and waits for sign-off, an assistant you text from your phone, and agents that review every
pull request are not three different systems. They are the same few pieces, combined
differently.

## Next up

In **[Part 4 — Running Orka for real](04-running-orka-for-real.md)**, we look at it from the
operator's side: what Orka records, how to trace agents with OpenTelemetry, how to see what
they cost, and how Orka is built underneath.

**Read more:** [Scheduled tasks](https://orka-agents.github.io/orka/docs/guides/scheduled-tasks) ·
[Gateways](https://orka-agents.github.io/orka/docs/gateways) ·
[Multi-agent coordination](https://orka-agents.github.io/orka/docs/multi-agent-coordination) ·
[Repository monitors](https://orka-agents.github.io/orka/docs/repository-monitors)
