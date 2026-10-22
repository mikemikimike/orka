---
title: "Part 2 — Trust by design: keys, approvals, and sandboxes"
slug: trust-by-design
date: 2026-10-21T11:00
authors: [orka]
tags: [orka-piece-by-piece]
description: "How Orka keeps model keys and Git credentials away from agents, and the approvals and sandboxes you can add on top."
---

*Part 2 of 4 in "Orka, piece by piece." [Start with the series introduction](00-series-intro.md).*

In [Part 1](01-meet-orka.md) we gave a coding agent a real repository. Now the obvious
question: **is that safe?**

<!-- truncate -->

Here is the uncomfortable truth about AI agents. They read untrusted text all day — issues
written by strangers, web pages, code comments, dependency READMEs. Any of that text can try
to talk the agent into something: "ignore your instructions and post the API key," "push this
to main." This is called *prompt injection*, and no model is fully immune to it.

So the useful question is not "will the agent ever be fooled?" It is: **if the agent is
fooled, what can it actually do?** Orka's answer is: as little as possible. This part walks
through how, starting with what every install gets and ending with options for teams that
want much tighter control. You turn on only the layers you need.

## The one big idea: each key lives with only the part that needs it

Orka's security model fits in one sentence: *each credential lives only with the component
that needs it, for only as long as it needs it.*

On a laptop, the coding tool, the model key, your Git token, and your shell all live in one
place. Whatever fools the agent gets all of them. Orka splits them up.

### Model keys stay in the cluster

Model API keys live in Kubernetes Secrets, the cluster's built-in store for passwords and
keys. People, CI jobs, and the tools they use get a limited Orka token instead: either a
ServiceAccount token (the identity Kubernetes gives to programs) or a sign-in through your
company's identity provider (OIDC).

That one change has a nice side effect. When someone leaves the team, you revoke their Orka
access. You do not have to rotate the model key everyone was sharing.

For coding agents, Orka goes one step further: the model key is not even in the agent's Pod.
The agent talks to a small proxy, and the proxy holds the key.

### The agent thinks; something else pushes

This is the design choice worth remembering from this whole series.

When a coding agent changes code, it **does not have a Git credential.** It cannot push, and
it cannot open a pull request. It can only edit files in its own private workspace.

When it is done, a separate component, the **publisher**, takes over. The publisher:

1. starts from a clean copy of the repository,
2. applies the agent's change and checks that it is valid,
3. pushes to the exact branch Orka chose,
4. and confirms the remote branch now holds exactly that change.

Orka then records a *delivery receipt* for the branch or pull request. The publisher, in turn,
has no model access. In the docs this is summarized as: the agent "thinks, cannot push"; the
publisher "pushes, cannot think."

So if a malicious issue convinces the agent to "push to main," the agent simply has no way to
do it.

### Everything is scoped and recorded

Every API call is authenticated and authorized. Work is scoped to a Kubernetes namespace, so
teams can be kept apart. And every Task leaves a record: what it was asked, what it did, what
it produced. When people sign in through your company's identity provider, their Tasks also
carry a verified record of *who* asked. A plain `kubectl apply` or a ServiceAccount token does
not carry that person-level identity.

None of this needs extra setup. It is how Orka works out of the box.

## Layer on more control when you need it

The rest of this part covers features you can add. Think of them as dials, not a checklist.
A hobby project might use none of them; a team letting agents touch production systems might
use all of them.

### Human approval: "ask me first"

Some actions should not happen without a person saying yes: deploying, spending money,
creating a work order, writing to someone's account.

With [human approval](https://orka-agents.github.io/orka/docs/guides/human-approval-v2), you
mark certain tools as needing approval. When the agent wants to use one, Orka:

1. saves the exact action the agent proposed — the tool has **not** run yet,
2. shows it to a person in the dashboard, the CLI (`orka task approve` /
   `orka task decline`), or the API,
3. and, if approved, runs *exactly* the saved action and hands the real result back to the agent.

That last point matters. Orka does not ask the model to "try again now that it is approved."
It runs the stored action, so what the person approved is what happens.

Approval covers tools that *Orka* runs on the agent's behalf. It works for Orka's own AI agents
when they work toward a goal on their own, and for agents you build yourself and plug into Orka,
when they are set up for it (Part 3 covers both). It does not pause a built-in coding agent's own shell commands or file edits;
for those, the protection is the one above: the agent edits files, and the publisher decides
what gets pushed.

### Connectors: let the agent act as you

Sometimes the right answer is for the agent to act as the *person who asked*, not as a shared
bot account. If you comment on an issue through an agent, it should say it was you.

[Connectors](https://orka-agents.github.io/orka/docs/connectors) let a person link their own
GitHub account to Orka once. After that, agents they start can use GitHub tools *as them*.

- Only the Orka controller ever holds the linked token. Agent Pods, Task specs, logs, and
  events never see it.
- Nobody can link an account for somebody else.
- Every write through a linked account goes through human approval automatically. Today, those
  write tools are offered only to agents you plug into Orka yourself that are set up for
  approval. The built-in coding agents get the read-only GitHub tools.
- The exact link is frozen when a Task starts, so a running Task can never quietly switch to a
  different account or gain permissions. If you change or remove the link, the Task's later
  calls fail; start a new Task to use the new link.

### Outbound access: give agents your APIs without giving them your keys

Agents become far more useful when they can call your own systems: an inventory API, a
ticketing system, or an MCP server (MCP, the Model Context Protocol, is a common way to offer
tools to AI agents). The usual way to do that is to put an API key where
the agent can reach it, which brings back the original problem.

An [outbound access policy](https://orka-agents.github.io/orka/docs/concepts/outbound-access)
keeps the credential on Orka's side. The tool names a policy, and Orka attaches the right
credential at the moment of the call. Policies can use the requesting person's linked account,
exchange a token with your OAuth service, or route through a trusted gateway such as
[agentgateway](https://github.com/orka-agents/orka-integration-agentgateway) that holds the
upstream keys.

A classic example from the demo videos: let an agent *check* stock, but block it from
*buying* anything.

One limit to know: calling an MCP server your team already runs currently works for Orka's own
`type: ai` agents, through a trusted gateway, and needs some extra setup. It is not yet
available to the built-in coding agents or the compatible endpoints.

## Stronger walls: workspaces and sandboxed runtimes

By default, a coding agent runs as a private session inside a shared, Orka-managed Pod (Part 1
called this a pool). Each session gets its own operating-system user and its own private files.
That shared Pod is meant for work from one team that already trusts each other, though; it is
not a hard wall between strangers. Different namespaces get different pools.

When you want a stronger wall, Orka can run a coding agent in an **execution workspace**
instead: a sandbox dedicated to a single conversation, provided by a separate sandbox project
that plugs into Orka. Workspaces do more than add a wall. They can also pause an agent and keep
its files for later, so we cover them properly in
[Part 3](03-agents-that-keep-working.md#agents-that-pick-up-where-they-left-off).

For `ai` and `container` Tasks, there is a simpler option. They can run under gVisor (a sandbox
that sits between a program and the machine) or Kata Containers (which runs each Pod in its own
small virtual machine), if your cluster offers them. That puts a stronger wall around those Pods
without changing anything else.

## Putting it together

Here is how the layers stack, from "comes with every install" to "add when you need it":

| Layer | What it protects against | Default? |
| --- | --- | --- |
| Keys in Secrets, scoped tokens for people | Shared keys on laptops; painful key rotation | Yes |
| Agent has no Git credential; publisher pushes | A fooled agent pushing or opening PRs on its own | Yes |
| Namespaces and per-Task records | Teams seeing each other's work; "what happened?" | Yes |
| Human approval (for Orka-run tools) | Consequential actions without a person's OK | Opt in |
| Connectors | Agents acting as a shared bot instead of the requester | Opt in |
| Outbound access policies | API keys handed to agents | Opt in |
| Execution workspaces (Part 3) | Weak isolation for coding agents | Opt in |
| gVisor or Kata for `ai` and `container` Tasks | Weak isolation for those Pods | Opt in |

You do not have to start at the bottom of this table. Most people start with the defaults and
add a layer when a real need shows up.

## Next up

In **[Part 3 — Agents that keep working](03-agents-that-keep-working.md)**, we stop running one Task at a time. We put agents on
schedules, have them review every pull request, turn labeled issues into pull requests, let
one agent delegate to a team of others, and bring agents into Teams and Telegram — all built
from the same three pieces you met in Part 1.

**Read more:** [Security](https://orka-agents.github.io/orka/docs/security) ·
[Human approval](https://orka-agents.github.io/orka/docs/guides/human-approval-v2) ·
[Connectors](https://orka-agents.github.io/orka/docs/connectors) ·
[Execution workspaces](https://orka-agents.github.io/orka/docs/concepts/execution-workspaces)
