# Agent Skill Registry

Agent Skill Registry is the internal ADP domain for versioned Skills that FDEs can reference, publish, inherit from, and deploy into customer environments.

## Language

**Skill**:
A versioned, signed package that carries skill metadata, dependencies, and executable payloads.
_Avoid_: capability, plugin, tool

**Skill Namespace**:
The ownership and visibility boundary that scopes Skill identifiers in ADP and customer environments.
_Avoid_: folder, group

**Skill Package**:
The buildable source artifact produced in ADP before signing and publishing.
_Avoid_: source, project

**Published Skill**:
A signed, immutable Skill version that is available for resolution and deployment.
_Avoid_: release, latest skill
Published Skill versions are immutable and multiple versions of the same Skill may coexist.

**Local Published Skill**:
A customer-scoped Published Skill created and signed inside a customer environment for use only in that environment.
_Avoid_: customer skill, private skill

**Skill Draft**:
An untrusted Skill candidate created by an FDE or running Agent before approval, build, scanning, and signing.
_Avoid_: generated skill, temporary skill
Skill Drafts can be created by FDEs or running Agents, but they are not deployable until approved and signed.

**Agent Profile**:
The deployable package produced by ADP that declares runtime configuration and Skill dependencies for an Agent.

**Internal Skill**:
A Skill owned and published by Z company for reuse inside ADP.
_Avoid_: in-house skill, company skill

**Community Skill**:
An external Skill sourced from a community registry or marketplace and ingested into ADP.
_Avoid_: public skill, third-party skill

**Ingested Skill**:
A Community Skill version captured by ADP with locked provenance, policy checks, and Z company signing before it can be published.
_Avoid_: mirrored skill, imported skill

**Skill Dependency**:
A declared reference from one Skill to another Skill version or version range.

**Skill Lockfile**:
A resolved dependency manifest that freezes every Skill version, digest, signature chain, and SBOM reference required by an Agent Profile or Local Published Skill.
_Avoid_: dependency file, requirements file

**Skill Runtime Mode**:
The execution boundary assigned to a Published Skill, such as in-process loading or isolated execution.
_Avoid_: runtime type, plugin mode

**Skill Runtime Interface**:
The invocation contract a Runtime Payload implements so Agents can call Skills regardless of implementation language.
_Avoid_: plugin API, skill API

**Skill Permission Manifest**:
A declaration of the data, network, filesystem, secret, model, tool, and cluster permissions required by a Published Skill.
_Avoid_: permission config, access list

**Secret Reference**:
A symbolic declaration of a secret a Skill needs, resolved only inside a customer environment.
_Avoid_: secret value, credential

**Offline Deployment Bundle**:
A customer-bound export from ADP that contains an Agent Profile, its Skill Lockfile, required Skill artifacts, signatures, SBOM references, and policy snapshot.
_Avoid_: install package, release bundle

**Policy Snapshot**:
The versioned policy bundle exported by ADP and enforced by the customer-environment Controller.
_Avoid_: policy config, rules

**Revocation List**:
A signed list of Skill versions or signing credentials that must no longer be admitted or hot-loaded.
_Avoid_: denylist, blacklist

**Local Skill Registry**:
The customer-environment registry that stores trusted Published Skill artifacts imported from an Offline Deployment Bundle.
_Avoid_: local cache, file store

**Air-Gapped Skill Distribution**:
The offline distribution path that moves signed Skill artifacts, Agent Profiles, Skill Lockfiles, Policy Snapshots, SBOMs, provenance, and Revocation Lists from ADP into a customer environment without network connectivity.
_Avoid_: upload, copy, sync

**Offline Signing Component**:
A Z-authorized customer-environment component that signs Local Published Skills within a limited customer scope.
_Avoid_: local CA, customer signer

**Skill Control Plane**:
The ADP-side platform layer that governs Skill authoring, ingestion, evaluation, signing, publishing, dependency resolution, and offline export.
_Avoid_: registry backend, marketplace service

**Skill Data Plane**:
The customer-environment runtime layer that imports, verifies, mounts, retrieves, hot-loads, invokes, traces, and revokes Skills.
_Avoid_: runtime system, customer service

**Agent Skill Operator**:
The customer-environment Kubernetes Operator that reconciles Agent Profile declarations, Skill Lockfiles, Local Skill Registry artifacts, policy, and revocation state into mounted and verified Skills for Agent Pods.
_Avoid_: controller, sync job

**Skill Cache**:
The node-local or cluster-local cache of verified Skill artifacts and unpacked payloads used by the Agent Skill Operator to reduce Pod startup latency and support deterministic hot-load.
_Avoid_: temp dir, download cache

**Skill Sidecar Runtime**:
An isolated process in the Agent Pod that loads and invokes higher-risk or dependency-heavy Skills through the Skill Runtime Interface while enforcing local permission and trace contracts.
_Avoid_: helper container, plugin server

**Skill Loader**:
The Agent Runtime module that discovers mounted Skills, verifies local metadata, performs compatibility checks, resolves task-time retrieval results, loads Runtime Payloads, and switches versions.
_Avoid_: importer, dynamic loader

**Semantic Skill Retrieval**:
The task-time retrieval process that indexes Published Skill metadata, descriptions, schemas, examples, permission summaries, and evaluation summaries so the Agent loads only relevant Skills within its context budget.
_Avoid_: search, recommendation

**Progressive Skill Disclosure**:
The runtime policy that exposes only compact Skill cards first, then loads full manifests, examples, assets, and Runtime Payloads only when a Skill is selected for use.
_Avoid_: lazy loading, context trimming

**Skill Trust Level**:
The governance classification assigned to a Skill source and lifecycle state, such as Official, Internal, Reviewed Community, Unreviewed Community, Agent Generated, or Customer Local.
_Avoid_: rating, category

**Skill Workbench**:
The ADP workspace where FDEs create, test, evaluate, and submit Skills through guided workflows.
_Avoid_: console, editor, playground

**Skill Evaluation**:
The curated test and review evidence used to qualify a Skill for publication or reuse.
_Avoid_: benchmark, scorecard

**Skill Invocation Trace**:
A customer-local observability record for a Skill call, keyed by Agent Profile, Skill version, digest, permissions, latency, and outcome.
_Avoid_: log, telemetry event

**Tenant Admin**:
The customer-side operator authorized to approve imports into the Local Skill Registry.
_Avoid_: customer admin, tenant operator

**Skill Manifest**:
The declarative identity and policy layer for a Skill, including version, dependencies, permissions, and runtime mode.

**Compatibility Contract**:
The versioned runtime, platform, and permission schema requirements declared by a Skill.
_Avoid_: compatibility matrix, support policy

**Runtime Payload**:
The executable code or rule set carried by a Skill.

**Skill Assets**:
The prompts, templates, examples, and knowledge references bundled with a Skill.

**Evaluation Artifacts**:
The datasets, judges, scripts, and regression baselines used to evaluate a Skill.

**Knowledge Asset**:
An external knowledge source that a Skill references instead of embedding directly.

**Controller**:
The Kubernetes control-plane component that reconciles an Agent Profile into a running workload and mounts its Skills.

**Agent Pod**:
The Pod that runs the deployed Agent workload.

## Relationships

- A **Skill Package** builds into one or more **Published Skill** versions
- A **Skill** belongs to exactly one **Skill Namespace**
- A **Skill Draft** may become a **Published Skill** only after approval, build, scanning, and signing
- FDEs and running Agents may create **Skill Drafts**, but only approved and signed drafts become deployable
- A customer-environment **Skill Draft** may become a **Local Published Skill** after local evaluation, Tenant Admin approval, and signing by the **Offline Signing Component**
- The **Skill Control Plane** exports **Air-Gapped Skill Distribution** bundles for the customer **Skill Data Plane**
- The **Skill Data Plane** admits only artifacts imported into the **Local Skill Registry** or already present in the **Skill Cache**
- A **Local Published Skill** may depend only on **Published Skills** or **Local Published Skills** already present in the same **Local Skill Registry**
- A **Local Published Skill** must not depend directly on a **Community Skill**
- A **Community Skill** must become an **Ingested Skill** before it can become a **Published Skill**
- A **Skill Trust Level** is derived from the source, signing identity, approval evidence, and customer scope
- A **Published Skill** may depend on zero or more **Skill Dependency** entries
- A **Published Skill** declares a **Skill Runtime Mode**
- A **Published Skill** declares a **Skill Permission Manifest**
- A **Skill Permission Manifest** may declare **Secret References**, but Skills do not carry secret values
- A **Runtime Payload** implements the **Skill Runtime Interface**
- A **Skill Manifest** declares a **Compatibility Contract**
- Published Skill versions are immutable and multiple versions of the same Skill may coexist
- An **Agent Profile** declares one or more **Skill** dependencies by identifier and version constraint
- An **Agent Profile** must include a **Skill Lockfile** before it can be deployed to a customer environment
- An **Offline Deployment Bundle** carries an **Agent Profile** and every **Published Skill** version named by its **Skill Lockfile**
- An **Offline Deployment Bundle** carries the **Policy Snapshot** used to evaluate its Skills in the customer environment
- A customer environment uses a local **Revocation List** during admission and hot-loading
- A **Local Skill Registry** stores the **Published Skill** versions imported from an **Offline Deployment Bundle**
- A **Local Skill Registry** may also store **Local Published Skills** that are valid only in that customer environment
- An FDE uses the **Skill Workbench** as the primary path to create, test, evaluate, and submit Skills
- A **Published Skill** is not reusable until it passes a required **Skill Evaluation**
- A **Tenant Admin** approves imports into the **Local Skill Registry**
- A running Agent emits a **Skill Invocation Trace** for each Skill call
- A **Skill** is composed of a **Skill Manifest**, a **Runtime Payload**, **Skill Assets**, and **Evaluation Artifacts**
- A **Skill Asset** may reference a **Knowledge Asset** instead of embedding large knowledge content directly
- A **Controller** resolves and mounts the required **Published Skill** versions into the **Agent Pod**
- An **Agent Skill Operator** is the production form of the customer-environment **Controller**
- An **Agent Skill Operator** verifies and unpacks Skills into the **Skill Cache** before exposing them to an **Agent Pod**
- A customer-environment **Controller** pulls only from the **Local Skill Registry** and local cache
- A customer-environment **Controller** follows the **Skill Lockfile** and does not resolve version ranges
- A **Skill Loader** uses **Semantic Skill Retrieval** and **Progressive Skill Disclosure** to select relevant mounted Skills at task time
- A **Skill Sidecar Runtime** invokes isolated Skills through the **Skill Runtime Interface**
- A running Agent may hot-load only **Published Skill** versions that are already mounted and signature-verified locally

## Example dialogue

> **Dev:** "Can an **Agent Profile** reference a **Community Skill** directly?"
> **Domain expert:** "Yes, but only through a resolved and signed **Published Skill** version."
