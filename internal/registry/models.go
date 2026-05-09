package registry

import "time"

type State struct {
	Drafts          map[string]*SkillDraft     `json:"drafts"`
	Published       map[string]*PublishedSkill `json:"published"`
	Revocations     map[string]*Revocation     `json:"revocations"`
	Traces          []SkillInvocationTrace     `json:"traces"`
	AuditEvents     []AuditEvent               `json:"audit_events"`
	ImportedBundles map[string]ImportedBundle  `json:"imported_bundles"`
}

type SkillDraft struct {
	ID                 string             `json:"id"`
	Namespace          string             `json:"namespace"`
	Name               string             `json:"name"`
	Version            string             `json:"version"`
	Description        string             `json:"description"`
	Visibility         string             `json:"visibility"`
	Source             string             `json:"source"`
	Status             string             `json:"status"`
	CreatedBy          string             `json:"created_by"`
	RuntimePayload     RuntimePayload     `json:"runtime_payload"`
	Dependencies       []SkillDependency  `json:"dependencies"`
	PermissionManifest PermissionManifest `json:"permission_manifest"`
	Assets             SkillAssets        `json:"assets"`
	Evaluation          SkillEvaluation    `json:"evaluation"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

type CreateDraftRequest struct {
	Namespace          string             `json:"namespace"`
	Name               string             `json:"name"`
	Version            string             `json:"version"`
	Description        string             `json:"description"`
	Visibility         string             `json:"visibility"`
	Source             string             `json:"source"`
	CreatedBy          string             `json:"created_by"`
	RuntimePayload     RuntimePayload     `json:"runtime_payload"`
	Dependencies       []SkillDependency  `json:"dependencies"`
	PermissionManifest PermissionManifest `json:"permission_manifest"`
	Assets             SkillAssets        `json:"assets"`
	Evaluation          SkillEvaluation    `json:"evaluation"`
}

type RuntimePayload struct {
	Mode      string `json:"mode"`
	Interface string `json:"interface"`
	Kind      string `json:"kind"`
	Entrypoint string `json:"entrypoint"`
	Template  string `json:"template"`
}

type SkillDependency struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type PermissionManifest struct {
	DataDomains   []string          `json:"data_domains"`
	Network       NetworkPermission `json:"network"`
	Filesystem    FilePermission    `json:"filesystem"`
	Secrets       []SecretReference `json:"secrets"`
	Models        []ModelPermission `json:"models"`
	Kubernetes    KubernetesAccess  `json:"kubernetes"`
	DraftCreation DraftCreation     `json:"draft_creation"`
}

type NetworkPermission struct {
	Egress []NetworkEgress `json:"egress"`
}

type NetworkEgress struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Ports  []int  `json:"ports"`
}

type FilePermission struct {
	Read  []string `json:"read"`
	Write []string `json:"write"`
}

type SecretReference struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Value    string `json:"value,omitempty"`
}

type ModelPermission struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

type KubernetesAccess struct {
	APIAccess bool `json:"api_access"`
}

type DraftCreation struct {
	Allowed bool `json:"allowed"`
}

type SkillAssets struct {
	Prompts         []string         `json:"prompts"`
	Templates       []string         `json:"templates"`
	Examples        []string         `json:"examples"`
	KnowledgeRefs   []KnowledgeAsset `json:"knowledge_refs"`
	SmallReferences []string         `json:"small_references"`
}

type KnowledgeAsset struct {
	Name    string `json:"name"`
	URI     string `json:"uri"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type SkillEvaluation struct {
	Cases      []EvaluationCase   `json:"cases"`
	LastResult *EvaluationResult  `json:"last_result,omitempty"`
}

type EvaluationCase struct {
	Name             string            `json:"name"`
	Input            map[string]string `json:"input"`
	ExpectedContains []string          `json:"expected_contains"`
}

type EvaluationResult struct {
	Passed       bool                 `json:"passed"`
	Score        float64              `json:"score"`
	CaseResults  []EvaluationCaseRun  `json:"case_results"`
	FailureCount int                  `json:"failure_count"`
	Warnings     []string             `json:"warnings"`
	RanAt        time.Time            `json:"ran_at"`
}

type EvaluationCaseRun struct {
	Name   string `json:"name"`
	Output string `json:"output"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

type PublishedSkill struct {
	ID                 string             `json:"id"`
	Namespace          string             `json:"namespace"`
	Name               string             `json:"name"`
	Version            string             `json:"version"`
	Description        string             `json:"description"`
	Visibility         string             `json:"visibility"`
	Source             string             `json:"source"`
	Status             string             `json:"status"`
	Local              bool               `json:"local"`
	Digest             string             `json:"digest"`
	ArtifactRef        string             `json:"artifact_ref"`
	Signature          string             `json:"signature"`
	SignatureKeyID     string             `json:"signature_key_id"`
	SignatureScope     string             `json:"signature_scope"`
	RuntimePayload     RuntimePayload     `json:"runtime_payload"`
	Dependencies       []SkillDependency  `json:"dependencies"`
	PermissionManifest PermissionManifest `json:"permission_manifest"`
	Assets             SkillAssets        `json:"assets"`
	EvaluationResult   EvaluationResult   `json:"evaluation_result"`
	SBOM               SBOM               `json:"sbom"`
	Provenance         Provenance         `json:"provenance"`
	PublishedAt        time.Time          `json:"published_at"`
}

type SBOM struct {
	Format     string          `json:"format"`
	Components []SBOMComponent `json:"components"`
	GeneratedAt time.Time      `json:"generated_at"`
}

type SBOMComponent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
}

type Provenance struct {
	PredicateType string    `json:"predicate_type"`
	Builder       string    `json:"builder"`
	Source        string    `json:"source"`
	Materials     []string  `json:"materials"`
	BuiltAt       time.Time `json:"built_at"`
}

type SkillArtifact struct {
	MediaType          string             `json:"media_type"`
	ArtifactType       string             `json:"artifact_type"`
	SchemaVersion      string             `json:"schema_version"`
	Skill              PublishedSkill     `json:"skill"`
	PermissionManifest PermissionManifest `json:"permission_manifest"`
	SBOM               SBOM               `json:"sbom"`
	Provenance         Provenance         `json:"provenance"`
}

type AgentProfileRequest struct {
	ID      string                  `json:"id"`
	Version string                  `json:"version"`
	Skills  []AgentProfileSkillRef `json:"skills"`
}

type AgentProfileSkillRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type SkillLockfile struct {
	SchemaVersion  string           `json:"schema_version"`
	AgentProfile   AgentProfileInfo `json:"agent_profile"`
	GeneratedAt     time.Time        `json:"generated_at"`
	PolicySnapshot  string           `json:"policy_snapshot"`
	Skills          []LockedSkill    `json:"skills"`
}

type AgentProfileInfo struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type LockedSkill struct {
	ID                 string `json:"id"`
	Version            string `json:"version"`
	Artifact           string `json:"artifact"`
	Digest             string `json:"digest"`
	Signature          string `json:"signature"`
	SignatureKeyID     string `json:"signature_key_id"`
	RuntimeInterface   string `json:"runtime_interface"`
	RuntimeMode        string `json:"runtime_mode"`
	PermissionManifest string `json:"permission_manifest"`
	SBOM               string `json:"sbom"`
	Provenance         string `json:"provenance"`
}

type OfflineBundle struct {
	SchemaVersion  string                    `json:"schema_version"`
	ID             string                    `json:"id"`
	CreatedAt      time.Time                 `json:"created_at"`
	Lockfile       SkillLockfile             `json:"lockfile"`
	PolicySnapshot string                    `json:"policy_snapshot"`
	Revocations    []Revocation              `json:"revocations"`
	Skills         []PublishedSkill          `json:"skills"`
	Artifacts      map[string]SkillArtifact  `json:"artifacts"`
	Signature      string                    `json:"signature"`
}

type ImportedBundle struct {
	ID        string    `json:"id"`
	ImportedAt time.Time `json:"imported_at"`
	SkillCount int       `json:"skill_count"`
}

type Revocation struct {
	ID          string    `json:"id"`
	TargetType  string    `json:"target_type"`
	TargetDigest string    `json:"target_digest"`
	Reason      string    `json:"reason"`
	SignedBy    string    `json:"signed_by"`
	EffectiveAt time.Time `json:"effective_at"`
}

type InvokeRequest struct {
	AgentProfileID string            `json:"agent_profile_id"`
	SkillID        string            `json:"skill_id"`
	Version        string            `json:"version"`
	Input          map[string]string `json:"input"`
}

type InvokeResponse struct {
	Output string                `json:"output"`
	Trace  SkillInvocationTrace  `json:"trace"`
}

type SkillInvocationTrace struct {
	InvocationID       string    `json:"invocation_id"`
	AgentProfileID     string    `json:"agent_profile_id"`
	SkillID            string    `json:"skill_id"`
	SkillVersion       string    `json:"skill_version"`
	SkillDigest        string    `json:"skill_digest"`
	RuntimeMode        string    `json:"runtime_mode"`
	PermissionContext  string    `json:"permission_context_hash"`
	InputSummary       string    `json:"input_summary_redacted"`
	OutputSummary      string    `json:"output_summary_redacted"`
	LatencyMillis      int64     `json:"latency_ms"`
	ErrorCode          string    `json:"error_code,omitempty"`
	EvaluationTag      string    `json:"evaluation_tag"`
	Timestamp          time.Time `json:"timestamp"`
}

type AuditEvent struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Result    string    `json:"result"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

