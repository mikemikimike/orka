# 39. Give Agents a Microsoft Entra agent identity

Date: 2026-10-22

## Status

Proposed. Not implemented. Builds on ADR 0034 (Agent runtime ceilings) and
ADR 0033 (per-user connectors). Amends ADR 0011's "no central credential
broker" non-goal for exactly one credential kind. On-behalf-of tokens, Entra
agent user accounts, and native `ai` Agents stay out of scope.

## Context

ADR 0034 lets an Agent cap what a Task may make it do inside its runtime:
tools, Bash, and turns. It says nothing about what the Agent reaches outside
the cluster. Today an outbound Tool call authenticates as whatever its
`OutboundAccessPolicy` resolves: a Secret, a ServiceAccount token exchange,
the Task's transaction token, or the requesting person's Connection. None of
these names the Agent. A downstream service or an identity administrator
cannot tell which Agent acted, cannot grant one Agent a permission without
granting it to every Agent that shares the credential, and cannot switch off
one Agent.

Microsoft Entra Agent ID provides that principal
([overview](https://learn.microsoft.com/en-us/entra/agent-id/agent-identities),
[autonomous token flow](https://learn.microsoft.com/en-us/entra/agent-id/autonomous-agent-authentication-authorization-flow)):

- An *agent identity* is a service principal with no credentials of its own.
  Tokens issued for it carry it as the subject, and Entra sign-in logs record
  it. Its object ID and app ID are the same value.
- An *agent identity blueprint* holds the credential (Microsoft recommends a
  federated identity credential) and acquires tokens for every agent identity
  created from it. Administrators can apply Conditional Access to, disable, or
  revoke grants for all identities of one blueprint at once.
- An app-only token takes two requests. The blueprint authenticates with
  `grant_type=client_credentials`, `scope=api://AzureADTokenExchange/.default`,
  and `fmi_path=<agent identity client ID>`. That token is then the
  `client_assertion` for a second `client_credentials` request with
  `client_id=<agent identity client ID>` and the target resource's
  `/.default` scope.
- Permissions are app role assignments or admin consent on the agent
  identity. A `/.default` token carries every role already granted; the
  caller cannot narrow it per request.
- Agent identities are single-tenant.

Four facts about Orka today make a naive binding unsafe:

1. **Nobody authorizes the use of an Agent.** Creating a Task authorizes the
   Task, not the Agent it names. Workspace classes have a `use` check in the
   API (`internal/api/workspace_class_authorization.go`) and in admission
   (`internal/admission/workspace_class_use.go`); Agents have none. If an
   Agent carried an Entra identity, anyone who can create a Task in its
   namespace would act as that identity.
2. **The blueprint credential is a master key.** The caller chooses
   `fmi_path`, so whoever can present the blueprint's credential can mint
   tokens for any agent identity under it.
3. **Agent editors could claim someone else's identity.** An agent identity
   client ID is just a string. Without a check, an Agent in one namespace
   could name the identity provisioned for another team's Agent.
4. **The direct adapter cannot express the flow, and should not.**
   `DirectOutboundAccess` supports only `TokenExchange` and `JWTBearer` grants
   and `None`, client-secret, or `PrivateKeyJWT` client authentication
   (`api/v1alpha1/outboundaccesspolicy_types.go`). `fmi_path` is not a
   reserved `additionalParameters` key, so stretching that adapter to
   `client_credentials` would hand the choice of identity to whoever writes
   the policy.

Orka also defines `gen_ai.agent.id` (`internal/tracing/genai/constants.go`)
but does not emit it, so there is no stable Agent attribution to join with
Entra's logs.

## Decision

Land in three stages. Each ships on its own; stage 3 requires stage 1.

### Stage 1: authorize the use of an Agent

- Add the SubjectAccessReview verb `use` on `agents.core.orka.ai`, with the
  Agent's name as the resource name, mirroring workspace classes. The API
  checks the end user before it creates a Task; an admission webhook checks
  `AdmissionRequest.UserInfo` for direct Kubernetes writes.
- Orka's own creators (the API, gateways, trusted workers) hold `use` broadly,
  as the controller already holds `use` on workspace classes, and must check
  their own callers before naming an identity-bound Agent.
- The check is mandatory for every Agent that declares an identity: such an
  Agent is not Ready unless the webhook is serving, and a Task naming it fails
  closed otherwise. For other Agents it stays opt-in behind a flag, because
  turning it on breaks every caller that lacks a `use` grant.
- A delegated child Task may name an identity-bound Agent only when it is the
  parent's own Agent. Delegation targets have no ceiling yet (ADR 0034), so a
  child must not borrow an identity its root requester was never checked
  against.

### Stage 2: bind the identity and attribute actions to it

- Add `Agent.spec.identity.entra.agentIdentityID`, the agent identity's client
  ID. The tenant and blueprint are not on the Agent; they are controller
  configuration, one blueprint per Orka installation.
- Setting or changing the field requires the SubjectAccessReview verb `bind`
  on the virtual resource `entraagentidentities.core.orka.ai`, with the client
  ID as the resource name, in the Agent's namespace. Operators grant it with a
  Role listing exact `resourceNames`. This is the pattern Kubernetes uses for
  binding Roles, and it settles who may claim which identity without a new
  CRD.
- One identity maps to one Agent UID. A second Agent that claims an identity
  already bound elsewhere reports `Ready=False`.
- The Task binding freezes the identity next to the Agent UID and generation
  that `AgentExecutionAgentRef` already pins. Editing the Agent does not change
  the identity a bound Task uses.
- Emit `gen_ai.agent.id` as the Agent UID, not its reusable name, on Task and
  Tool-call spans, and record the agent identity ID on the audit record of
  every external effect made with it. Entra sign-in logs and Orka records then
  share a join key.
- Until stage 3 ships, the identity is a declaration by an authorized Agent
  editor, not a verified fact, and status and UI must say so.

### Stage 3: issue agent identity tokens from the controller

- Add a fourth `OutboundAccessPolicy` adapter, `agentIdentity`, exclusive with
  `direct`, `gateway`, and `connection`. It names the target resource (for
  example `https://graph.microsoft.com`) and the output header. It never names
  an identity: the identity always comes from the Task's frozen Agent binding,
  not from the policy, the Tool, or the Task spec.
- Only the controller implements the credential source, as with Connections,
  whose source "exists only in the controller; worker Pods have no
  implementation" (`internal/outboundaccess/resolver.go`). ACP Tasks reach it
  through the controller's MCP broker (`internal/controller/acp_mcp_broker.go`).
  Native `ai` workers and every other process fail closed.
- The controller authenticates as the blueprint with its own projected
  ServiceAccount token as the federated credential, then performs the two
  requests above. The blueprint's federated identity credential trusts
  exactly the controller ServiceAccount's subject. No other workload may run
  as that ServiceAccount, and nothing that can create Pods or Jobs, such as the
  native `ai` worker (`config/rbac/worker_role.yaml`), may do so in the
  controller's namespace.
- Tokens are injected per Tool call. They never enter the ACP process tree,
  the Task spec or status, the execution snapshot, events, logs, or metrics.
  The cache is keyed by a digest of the agent identity, resource, and policy
  UID and generation, and an entry lives at most five minutes. Because nothing
  else holds the token, disabling the identity in Entra stops new calls
  within that bound.
- Reserve `fmi_path`, `user_id`, `username`, and
  `user_federated_identity_credential` in `DirectOutboundAccess`
  `additionalParameters`, so the direct adapter can never select an identity.
- App-only tokens only. On-behalf-of tokens and agent user accounts act for a
  person; they belong with Connections (ADR 0033) and remain an ADR 0011
  non-goal.
- An identity is verified only after the controller has acquired a token for
  it. That proves the identity is a child of the configured blueprint, which
  stage 2 cannot.

### Relationship to ADR 0034

The ceiling and the identity bound different things and stay separate checks.
The ceiling bounds what a Task may make the Agent do inside its runtime; Entra
permissions bound what the identity may do outside it. Neither can be derived
from the other: one lists tool names, the other app roles. An Agent owner who
wants to limit which identity-backed resources a Task may reach can later add
`ceiling.agentIdentityResources`, checked at binding like the rest of the
ceiling. Narrowing app roles stays an Entra administrator's job.

## Rejected alternatives

- **A per-Agent ServiceAccount federated to its agent identity.** Agent
  identities cannot hold credentials; only the blueprint can. A per-Agent
  ServiceAccount is also mintable by anyone who can create a Pod naming it.
- **The blueprint credential in the runtime Pod or a sidecar**, as in the
  Microsoft Entra SDK sidecar pattern. The runtime runs agent-controlled code,
  and the blueprint credential impersonates every identity under it.
- **Extending the direct adapter** with `client_credentials`, a ServiceAccount
  client assertion, and `fmi_path` as a parameter. That turns impersonation
  into a policy author's choice.
- **Deriving the ceiling from Entra permissions**, or the reverse. They do not
  share a vocabulary.

## Consequences

- An Agent becomes a principal outside Orka. Entra administrators can grant,
  scope, disable, and audit each Agent, and downstream services see which
  Agent called.
- The controller becomes a credential broker for one credential kind, which
  amends ADR 0011; on acceptance, 0011's status points here. A compromised
  controller already exposes Connection tokens; it would now also expose every
  agent identity under the installation's blueprint. Stronger separation needs
  separate blueprints, which means separate Orka installations.
- ADR 0011 keeps provider specifics out of core. The adapter is shaped as
  `agentIdentity.entra` so another issuer can sit beside it, but the
  two-request blueprint flow is Entra-specific and lives in core.
- Entra must reach the cluster's ServiceAccount issuer. AKS publishes it;
  kind and other private clusters must publish their discovery document and
  JWKS.
- An Entra outage fails identity-backed Tool calls closed, with no fallback
  to another credential.
- Requiring `use` for all Agents is a breaking change and stays opt-in.
- Not covered: inbound tokens addressed to an agent identity, multi-tenant
  blueprints, native `ai` Agents, and delegation to any Agent other than the
  parent's own.
