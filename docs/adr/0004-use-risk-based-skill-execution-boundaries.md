# Use risk-based Skill execution boundaries

Published Skills default to in-process loading for developer ergonomics and low latency, but higher-risk Skills must use an isolated Skill Runtime Mode such as a sidecar or sandbox. This avoids making every Skill operationally heavy while preserving a path to stronger containment for Skills with sensitive data access, external effects, or untrusted provenance.
