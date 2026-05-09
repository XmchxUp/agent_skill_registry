package registry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	draftStatusDraft         = "draft"
	draftStatusEvaluated     = "evaluated"
	draftStatusPublished     = "published"
	publishedStatus          = "published"
	ingestionStatusFailed    = "failed"
	ingestionStatusPublished = "published"
	mountStatusReady         = "ready"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrInvalid  = errors.New("invalid request")
)

type Service struct {
	mu          sync.Mutex
	dataDir     string
	statePath   string
	artifactDir string
	signingKey  string
	state       State
}

func NewService(dataDir, signingKey string) (*Service, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	artifactDir := filepath.Join(dataDir, "artifacts")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return nil, err
	}
	s := &Service{
		dataDir:     dataDir,
		statePath:   filepath.Join(dataDir, "state.json"),
		artifactDir: artifactDir,
		signingKey:  signingKey,
		state: State{
			Drafts:                  map[string]*SkillDraft{},
			Published:               map[string]*PublishedSkill{},
			CommunityIngestions:     map[string]*CommunityIngestion{},
			Revocations:             map[string]*Revocation{},
			ImportedRevocationLists: map[string]ImportedRevocationList{},
			MountPlans:              map[string]ControllerMountPlan{},
			Traces:                  []SkillInvocationTrace{},
			AuditEvents:             []AuditEvent{},
			ImportedBundles:         map[string]ImportedBundle{},
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	if len(s.state.Published) == 0 && len(s.state.Drafts) == 0 {
		if err := s.seed(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneState(s.state)
}

func (s *Service) CreateDraft(req CreateDraftRequest) (*SkillDraft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	draft, err := s.createDraftLocked(req)
	if err != nil {
		return nil, err
	}
	s.audit("fde", "draft.create", draft.ID, "ok", "created skill draft")
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneDraft(draft), nil
}

func (s *Service) CreateRuntimeDraft(req CreateDraftRequest) (*SkillDraft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !req.PermissionManifest.DraftCreation.Allowed {
		return nil, fmt.Errorf("%w: runtime draft creation permission is required", ErrInvalid)
	}
	req.Source = "runtime-agent"
	req.CreatedBy = "agent-runtime"
	req.PermissionManifest.DraftCreation.Allowed = false
	draft, err := s.createDraftLocked(req)
	if err != nil {
		return nil, err
	}
	s.audit("runtime", "draft.create", draft.ID, "ok", "runtime-created Skill Draft")
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneDraft(draft), nil
}

func (s *Service) IngestCommunitySkill(req CommunityIngestRequest) (*CommunityIngestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	ingestion := &CommunityIngestion{
		ID:            "ing-" + hashString(req.SourceURL + ":" + req.SourceVersion + ":" + req.SourceDigest)[:12],
		SourceURL:     req.SourceURL,
		SourceVersion: req.SourceVersion,
		SourceDigest:  req.SourceDigest,
		License:       req.License,
		Scan:          req.Scan,
		Status:        ingestionStatusFailed,
		RequestedAt:   now,
	}
	if s.state.CommunityIngestions == nil {
		s.state.CommunityIngestions = map[string]*CommunityIngestion{}
	}
	if _, ok := s.state.CommunityIngestions[ingestion.ID]; ok {
		return nil, fmt.Errorf("%w: community ingestion already exists", ErrConflict)
	}
	if err := validateCommunityIngestRequest(req); err != nil {
		ingestion.FailureReason = err.Error()
		s.state.CommunityIngestions[ingestion.ID] = ingestion
		s.audit("community-ingestion", "community.ingest", ingestion.ID, "deny", err.Error())
		_ = s.saveLocked()
		return cloneCommunityIngestion(ingestion), err
	}

	draftReq := req.Skill
	draftReq.Source = fmt.Sprintf("community:%s@%s#%s", req.SourceURL, req.SourceVersion, req.SourceDigest)
	draftReq.CreatedBy = defaultString(draftReq.CreatedBy, "community-ingestion")
	if draftReq.Visibility == "" {
		draftReq.Visibility = "company-wide"
	}
	draft, err := s.createDraftLocked(draftReq)
	if err != nil {
		ingestion.FailureReason = err.Error()
		s.state.CommunityIngestions[ingestion.ID] = ingestion
		s.audit("community-ingestion", "community.ingest", ingestion.ID, "deny", err.Error())
		_ = s.saveLocked()
		return cloneCommunityIngestion(ingestion), err
	}
	result := runEvaluation(draft.RuntimePayload, draft.Evaluation.Cases)
	draft.Evaluation.LastResult = &result
	draft.Status = draftStatusEvaluated
	draft.UpdatedAt = time.Now().UTC()
	if !result.Passed {
		ingestion.FailureReason = "community skill smoke evaluation failed"
		s.state.CommunityIngestions[ingestion.ID] = ingestion
		s.audit("community-ingestion", "community.ingest", ingestion.ID, "deny", ingestion.FailureReason)
		_ = s.saveLocked()
		return cloneCommunityIngestion(ingestion), fmt.Errorf("%w: %s", ErrInvalid, ingestion.FailureReason)
	}
	published, err := s.publishDraftLocked(draft.ID, false)
	if err != nil {
		ingestion.FailureReason = err.Error()
		s.state.CommunityIngestions[ingestion.ID] = ingestion
		s.audit("community-ingestion", "community.ingest", ingestion.ID, "deny", err.Error())
		_ = s.saveLocked()
		return cloneCommunityIngestion(ingestion), err
	}
	ingestion.Status = ingestionStatusPublished
	ingestion.PublishedSkillID = published.ID
	ingestion.CompletedAt = time.Now().UTC()
	s.state.CommunityIngestions[ingestion.ID] = ingestion
	s.audit("community-ingestion", "community.ingest", ingestion.ID, "ok", published.ID)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return cloneCommunityIngestion(ingestion), nil
}

func (s *Service) EvaluateDraft(id string) (*EvaluationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	draft, ok := s.state.Drafts[id]
	if !ok {
		return nil, ErrNotFound
	}
	result := runEvaluation(draft.RuntimePayload, draft.Evaluation.Cases)
	draft.Evaluation.LastResult = &result
	draft.Status = draftStatusEvaluated
	draft.UpdatedAt = time.Now().UTC()
	s.audit("ci", "draft.evaluate", id, boolResult(result.Passed), fmt.Sprintf("score %.2f", result.Score))
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) PublishDraft(id string, local bool) (*PublishedSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	published, err := s.publishDraftLocked(id, local)
	if err != nil {
		return nil, err
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return clonePublished(published), nil
}

func (s *Service) publishDraftLocked(id string, local bool) (*PublishedSkill, error) {

	draft, ok := s.state.Drafts[id]
	if !ok {
		return nil, ErrNotFound
	}
	if _, exists := s.state.Published[id]; exists {
		return nil, fmt.Errorf("%w: published skill already exists", ErrConflict)
	}
	if draft.Evaluation.LastResult == nil || !draft.Evaluation.LastResult.Passed {
		return nil, fmt.Errorf("%w: draft must pass evaluation before publication", ErrInvalid)
	}
	if err := validatePolicy(draft); err != nil {
		s.audit("policy", "draft.publish", id, "deny", err.Error())
		return nil, err
	}
	now := time.Now().UTC()
	sbom := buildSBOM(draft, now)
	provenance := Provenance{
		PredicateType: "https://slsa.dev/provenance/v1",
		Builder:       "agent-skill-registry-mvp",
		Source:        draft.Source,
		Materials:     dependencyIDs(draft.Dependencies),
		BuiltAt:       now,
	}
	published := PublishedSkill{
		ID:                 draft.ID,
		Namespace:          draft.Namespace,
		Name:               draft.Name,
		Version:            draft.Version,
		Description:        draft.Description,
		Visibility:         draft.Visibility,
		Source:             draft.Source,
		Status:             publishedStatus,
		Local:              local,
		RuntimePayload:     draft.RuntimePayload,
		Dependencies:       draft.Dependencies,
		PermissionManifest: stripSecretValues(draft.PermissionManifest),
		Assets:             draft.Assets,
		EvaluationResult:   *draft.Evaluation.LastResult,
		SBOM:               sbom,
		Provenance:         provenance,
		PublishedAt:        now,
	}
	artifact := SkillArtifact{
		MediaType:          "application/vnd.oci.image.manifest.v1+json",
		ArtifactType:       "application/vnd.z.adp.skill.v1+json",
		SchemaVersion:      "adp.skill.artifact/v1",
		Skill:              published,
		PermissionManifest: published.PermissionManifest,
		SBOM:               sbom,
		Provenance:         provenance,
	}
	digest, err := digestJSON(artifact)
	if err != nil {
		return nil, err
	}
	published.Digest = digest
	published.ArtifactRef = fmt.Sprintf("local-skill-registry/%s/%s@%s", published.Namespace, published.Name, digest)
	published.SignatureKeyID = "z-root-dev"
	if local {
		published.SignatureScope = "customer-local"
	} else {
		published.SignatureScope = "z-global"
	}
	published.Signature = s.sign(digest + ":" + published.SignatureScope)
	artifact.Skill = published
	if err := s.writeArtifactLocked(published.Digest, artifact); err != nil {
		return nil, err
	}
	draft.Status = draftStatusPublished
	draft.UpdatedAt = now
	s.state.Published[published.ID] = &published
	s.audit("platform", "skill.publish", id, "ok", published.SignatureScope)
	return clonePublished(&published), nil
}

func (s *Service) ListPublished() []PublishedSkill {
	return s.SearchPublished(SkillSearchFilter{})
}

func (s *Service) SearchPublished(filter SkillSearchFilter) []PublishedSkill {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]PublishedSkill, 0, len(s.state.Published))
	for _, skill := range s.state.Published {
		if filter.Namespace != "" && skill.Namespace != filter.Namespace {
			continue
		}
		if filter.Visibility != "" && skill.Visibility != filter.Visibility {
			continue
		}
		if filter.Source != "" && !strings.Contains(skill.Source, filter.Source) {
			continue
		}
		if filter.RuntimeMode != "" && skill.RuntimePayload.Mode != filter.RuntimeMode {
			continue
		}
		if filter.Query != "" {
			q := strings.ToLower(filter.Query)
			haystack := strings.ToLower(skill.ID + " " + skill.Description + " " + strings.Join(skill.PermissionManifest.DataDomains, " "))
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		items = append(items, *clonePublished(skill))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *Service) GetPublished(id string) (*PublishedSkill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	skill, ok := s.state.Published[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clonePublished(skill), nil
}

func (s *Service) ResolveLockfile(req AgentProfileRequest) (*SkillLockfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock, err := s.resolveLockfileLocked(req)
	if err != nil {
		return nil, err
	}
	s.audit("publisher", "agent_profile.resolve", req.ID+":"+req.Version, "ok", fmt.Sprintf("%d skills", len(lock.Skills)))
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return lock, nil
}

func (s *Service) ExportBundle(req AgentProfileRequest) (*OfflineBundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock, err := s.resolveLockfileLocked(req)
	if err != nil {
		return nil, err
	}
	artifacts := map[string]SkillArtifact{}
	skills := make([]PublishedSkill, 0, len(lock.Skills))
	for _, locked := range lock.Skills {
		skill := s.state.Published[locked.ID]
		if skill == nil {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, locked.ID)
		}
		artifact, err := s.readArtifactLocked(skill.Digest)
		if err != nil {
			return nil, err
		}
		artifacts[skill.Digest] = artifact
		skills = append(skills, *clonePublished(skill))
	}
	revocations := make([]Revocation, 0, len(s.state.Revocations))
	for _, rev := range s.state.Revocations {
		revocations = append(revocations, *rev)
	}
	sort.Slice(revocations, func(i, j int) bool {
		return revocations[i].ID < revocations[j].ID
	})
	bundle := &OfflineBundle{
		SchemaVersion:  "adp.offline_bundle/v1",
		ID:             "bundle-" + hashString(req.ID + ":" + req.Version + ":" + time.Now().UTC().Format(time.RFC3339Nano))[:12],
		CreatedAt:      time.Now().UTC(),
		Lockfile:       *lock,
		PolicySnapshot: lock.PolicySnapshot,
		Revocations:    revocations,
		Skills:         skills,
		Artifacts:      artifacts,
	}
	digest, err := digestJSON(bundleForSigning(*bundle))
	if err != nil {
		return nil, err
	}
	bundle.Signature = s.sign(digest + ":offline-bundle")
	s.audit("publisher", "offline_bundle.export", bundle.ID, "ok", fmt.Sprintf("%d skills", len(skills)))
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return bundle, nil
}

func (s *Service) ImportBundle(bundle OfflineBundle) (*ImportedBundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if bundle.SchemaVersion == "" || bundle.ID == "" {
		return nil, fmt.Errorf("%w: invalid bundle", ErrInvalid)
	}
	digest, err := digestJSON(bundleForSigning(bundle))
	if err != nil {
		return nil, err
	}
	if !s.verify(digest+":offline-bundle", bundle.Signature) {
		return nil, fmt.Errorf("%w: bundle signature verification failed", ErrInvalid)
	}
	for _, skill := range bundle.Skills {
		artifact, ok := bundle.Artifacts[skill.Digest]
		if !ok {
			return nil, fmt.Errorf("%w: missing artifact %s", ErrInvalid, skill.Digest)
		}
		if err := verifyArtifact(skill, artifact, s); err != nil {
			return nil, err
		}
		copySkill := skill
		s.state.Published[copySkill.ID] = &copySkill
		if err := s.writeArtifactLocked(copySkill.Digest, artifact); err != nil {
			return nil, err
		}
	}
	for _, rev := range bundle.Revocations {
		revCopy := rev
		s.state.Revocations[rev.ID] = &revCopy
	}
	imported := ImportedBundle{
		ID:         bundle.ID,
		ImportedAt: time.Now().UTC(),
		SkillCount: len(bundle.Skills),
	}
	s.state.ImportedBundles[bundle.ID] = imported
	s.audit("tenant-admin", "offline_bundle.import", bundle.ID, "ok", fmt.Sprintf("%d skills", len(bundle.Skills)))
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return &imported, nil
}

func (s *Service) PrepareControllerMount(req ControllerMountRequest) (*ControllerMountPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock := req.Lockfile
	if lock.SchemaVersion == "" {
		resolved, err := s.resolveLockfileLocked(req.AgentProfile)
		if err != nil {
			return nil, err
		}
		lock = *resolved
	}
	if lock.SchemaVersion != "adp.skill.lock/v1" {
		return nil, fmt.Errorf("%w: unsupported lockfile schema", ErrInvalid)
	}
	if lock.PolicySnapshot == "" {
		return nil, fmt.Errorf("%w: policy snapshot is required", ErrInvalid)
	}
	mounted := make([]MountedSkill, 0, len(lock.Skills))
	for _, locked := range lock.Skills {
		skill, ok := s.state.Published[locked.ID]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, locked.ID)
		}
		if skill.Version != locked.Version || skill.Digest != locked.Digest {
			return nil, fmt.Errorf("%w: lockfile digest mismatch for %s", ErrInvalid, locked.ID)
		}
		if s.isRevokedLocked(skill.Digest) {
			return nil, fmt.Errorf("%w: skill %s is revoked", ErrInvalid, locked.ID)
		}
		if !s.verify(skill.Digest+":"+skill.SignatureScope, skill.Signature) {
			return nil, fmt.Errorf("%w: skill signature verification failed", ErrInvalid)
		}
		artifact, err := s.readArtifactLocked(skill.Digest)
		if err != nil {
			return nil, err
		}
		if err := verifyArtifact(*skill, artifact, s); err != nil {
			return nil, err
		}
		mounted = append(mounted, MountedSkill{
			ID:                  skill.ID,
			Version:             skill.Version,
			Digest:              skill.Digest,
			MountPath:           mountPathForSkill(skill),
			ReadOnly:            true,
			RuntimeInterface:    skill.RuntimePayload.Interface,
			RuntimeMode:         skill.RuntimePayload.Mode,
			HotLoadReady:        true,
			PermissionResources: mapPermissionResources(skill),
		})
	}
	plan := ControllerMountPlan{
		SchemaVersion:  "adp.controller_mount_plan/v1",
		ID:             "mount-" + hashString(lock.AgentProfile.ID + ":" + lock.AgentProfile.Version + ":" + time.Now().UTC().Format(time.RFC3339Nano))[:12],
		AgentProfile:   lock.AgentProfile,
		GeneratedAt:    time.Now().UTC(),
		PolicySnapshot: lock.PolicySnapshot,
		Skills:         mounted,
		Status:         mountStatusReady,
	}
	if s.state.MountPlans == nil {
		s.state.MountPlans = map[string]ControllerMountPlan{}
	}
	s.state.MountPlans[plan.ID] = plan
	s.audit("controller", "agent_profile.mount", lock.AgentProfile.ID+":"+lock.AgentProfile.Version, "ok", fmt.Sprintf("%d skills", len(mounted)))
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return &plan, nil
}

func (s *Service) Invoke(req InvokeRequest) (*InvokeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := req.SkillID
	if req.Version != "" && !strings.Contains(req.SkillID, ":") {
		parts := strings.Split(req.SkillID, "/")
		if len(parts) == 2 {
			id = idFor(parts[0], parts[1], req.Version)
		}
	}
	skill, ok := s.state.Published[id]
	if !ok {
		return nil, ErrNotFound
	}
	if s.isRevokedLocked(skill.Digest) {
		return nil, fmt.Errorf("%w: skill is revoked", ErrInvalid)
	}
	if !s.verify(skill.Digest+":"+skill.SignatureScope, skill.Signature) {
		return nil, fmt.Errorf("%w: skill signature verification failed", ErrInvalid)
	}
	start := time.Now()
	output, err := renderPayload(skill.RuntimePayload, req.Input)
	trace := SkillInvocationTrace{
		InvocationID:      "inv-" + hashString(time.Now().UTC().Format(time.RFC3339Nano) + skill.ID)[:12],
		AgentProfileID:    defaultString(req.AgentProfileID, "local-agent"),
		SkillID:           skill.ID,
		SkillVersion:      skill.Version,
		SkillDigest:       skill.Digest,
		RuntimeMode:       skill.RuntimePayload.Mode,
		PermissionContext: hashPermission(skill.PermissionManifest),
		InputSummary:      summarizeMap(req.Input),
		OutputSummary:     summarizeString(output),
		LatencyMillis:     time.Since(start).Milliseconds(),
		EvaluationTag:     "runtime",
		Timestamp:         time.Now().UTC(),
	}
	if err != nil {
		trace.ErrorCode = "INVOKE_FAILED"
		trace.OutputSummary = ""
		s.state.Traces = append(s.state.Traces, trace)
		s.audit("runtime", "skill.invoke", skill.ID, "error", err.Error())
		_ = s.saveLocked()
		return nil, err
	}
	s.state.Traces = append(s.state.Traces, trace)
	s.audit("runtime", "skill.invoke", skill.ID, "ok", "invoked skill")
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return &InvokeResponse{Output: output, Trace: trace}, nil
}

func (s *Service) Revoke(targetDigest, reason string) (*Revocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rev, err := s.revokeLocked(targetDigest, reason)
	if err != nil {
		return nil, err
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return rev, nil
}

func (s *Service) ExportRevocationList() (*SignedRevocationList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	revocations := make([]Revocation, 0, len(s.state.Revocations))
	for _, rev := range s.state.Revocations {
		revocations = append(revocations, *rev)
	}
	sort.Slice(revocations, func(i, j int) bool {
		return revocations[i].ID < revocations[j].ID
	})
	list := &SignedRevocationList{
		SchemaVersion: "adp.revocation_list/v1",
		ID:            "revlist-" + hashString(time.Now().UTC().Format(time.RFC3339Nano))[:12],
		CreatedAt:     time.Now().UTC(),
		Revocations:   revocations,
	}
	digest, err := digestJSON(revocationListForSigning(*list))
	if err != nil {
		return nil, err
	}
	list.Signature = s.sign(digest + ":revocation-list")
	s.audit("security", "revocation_list.export", list.ID, "ok", fmt.Sprintf("%d revocations", len(revocations)))
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Service) ImportRevocationList(list SignedRevocationList) (*ImportedRevocationList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if list.SchemaVersion != "adp.revocation_list/v1" || list.ID == "" {
		return nil, fmt.Errorf("%w: invalid revocation list", ErrInvalid)
	}
	digest, err := digestJSON(revocationListForSigning(list))
	if err != nil {
		return nil, err
	}
	if !s.verify(digest+":revocation-list", list.Signature) {
		return nil, fmt.Errorf("%w: revocation list signature verification failed", ErrInvalid)
	}
	for _, rev := range list.Revocations {
		if rev.ID == "" || rev.TargetDigest == "" {
			return nil, fmt.Errorf("%w: revocation entries require id and target_digest", ErrInvalid)
		}
		revCopy := rev
		s.state.Revocations[rev.ID] = &revCopy
	}
	if s.state.ImportedRevocationLists == nil {
		s.state.ImportedRevocationLists = map[string]ImportedRevocationList{}
	}
	imported := ImportedRevocationList{
		ID:              list.ID,
		ImportedAt:      time.Now().UTC(),
		RevocationCount: len(list.Revocations),
	}
	s.state.ImportedRevocationLists[list.ID] = imported
	s.audit("tenant-admin", "revocation_list.import", list.ID, "ok", fmt.Sprintf("%d revocations", len(list.Revocations)))
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return &imported, nil
}

func (s *Service) revokeLocked(targetDigest, reason string) (*Revocation, error) {
	if targetDigest == "" {
		return nil, fmt.Errorf("%w: target_digest is required", ErrInvalid)
	}
	rev := &Revocation{
		ID:           "rev-" + hashString(targetDigest + ":" + time.Now().UTC().Format(time.RFC3339Nano))[:12],
		TargetType:   "skill_digest",
		TargetDigest: targetDigest,
		Reason:       defaultString(reason, "manual revocation"),
		SignedBy:     "z-root-dev",
		EffectiveAt:  time.Now().UTC(),
	}
	s.state.Revocations[rev.ID] = rev
	s.audit("security", "skill.revoke", targetDigest, "ok", rev.Reason)
	return rev, nil
}

func (s *Service) resolveLockfileLocked(req AgentProfileRequest) (*SkillLockfile, error) {
	if req.ID == "" || req.Version == "" {
		return nil, fmt.Errorf("%w: agent profile id and version are required", ErrInvalid)
	}
	if len(req.Skills) == 0 {
		return nil, fmt.Errorf("%w: at least one skill is required", ErrInvalid)
	}
	seen := map[string]bool{}
	var locked []LockedSkill
	var visit func(ref AgentProfileSkillRef) error
	visit = func(ref AgentProfileSkillRef) error {
		if ref.ID == "" || ref.Version == "" {
			return fmt.Errorf("%w: skill id and version are required", ErrInvalid)
		}
		id := ref.ID
		if !strings.Contains(id, ":") {
			parts := strings.Split(id, "/")
			if len(parts) != 2 {
				return fmt.Errorf("%w: skill id must be namespace/name", ErrInvalid)
			}
			id = idFor(parts[0], parts[1], ref.Version)
		}
		if seen[id] {
			return nil
		}
		skill, ok := s.state.Published[id]
		if !ok {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		if s.isRevokedLocked(skill.Digest) {
			return fmt.Errorf("%w: skill %s is revoked", ErrInvalid, id)
		}
		for _, dep := range skill.Dependencies {
			if err := visit(AgentProfileSkillRef{ID: dep.ID, Version: dep.Version}); err != nil {
				return err
			}
		}
		seen[id] = true
		locked = append(locked, LockedSkill{
			ID:                 skill.ID,
			Version:            skill.Version,
			Artifact:           skill.ArtifactRef,
			Digest:             skill.Digest,
			Signature:          skill.Signature,
			SignatureKeyID:     skill.SignatureKeyID,
			RuntimeInterface:   skill.RuntimePayload.Interface,
			RuntimeMode:        skill.RuntimePayload.Mode,
			PermissionManifest: hashPermission(skill.PermissionManifest),
			SBOM:               hashAny(skill.SBOM),
			Provenance:         hashAny(skill.Provenance),
		})
		return nil
	}
	for _, ref := range req.Skills {
		if err := visit(ref); err != nil {
			return nil, err
		}
	}
	sort.Slice(locked, func(i, j int) bool {
		return locked[i].ID < locked[j].ID
	})
	profileDigest := hashAny(req)
	return &SkillLockfile{
		SchemaVersion: "adp.skill.lock/v1",
		AgentProfile: AgentProfileInfo{
			ID:      req.ID,
			Version: req.Version,
			Digest:  "sha256:" + profileDigest,
		},
		GeneratedAt:    time.Now().UTC(),
		PolicySnapshot: "policy-mvp@sha256:" + hashString("policy-mvp-v1"),
		Skills:         locked,
	}, nil
}

func (s *Service) isRevokedLocked(digest string) bool {
	for _, rev := range s.state.Revocations {
		if rev.TargetType == "skill_digest" && rev.TargetDigest == digest {
			return true
		}
	}
	return false
}

func (s *Service) sign(message string) string {
	mac := hmac.New(sha256.New, []byte(s.signingKey))
	mac.Write([]byte(message))
	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil))
}

func (s *Service) verify(message, signature string) bool {
	return hmac.Equal([]byte(s.sign(message)), []byte(signature))
}

func (s *Service) load() error {
	data, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return err
	}
	if s.state.Drafts == nil {
		s.state.Drafts = map[string]*SkillDraft{}
	}
	if s.state.Published == nil {
		s.state.Published = map[string]*PublishedSkill{}
	}
	if s.state.CommunityIngestions == nil {
		s.state.CommunityIngestions = map[string]*CommunityIngestion{}
	}
	if s.state.Revocations == nil {
		s.state.Revocations = map[string]*Revocation{}
	}
	if s.state.ImportedRevocationLists == nil {
		s.state.ImportedRevocationLists = map[string]ImportedRevocationList{}
	}
	if s.state.MountPlans == nil {
		s.state.MountPlans = map[string]ControllerMountPlan{}
	}
	if s.state.ImportedBundles == nil {
		s.state.ImportedBundles = map[string]ImportedBundle{}
	}
	return nil
}

func (s *Service) saveLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath)
}

func (s *Service) writeArtifactLocked(digest string, artifact SkillArtifact) error {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	name := strings.TrimPrefix(digest, "sha256:") + ".json"
	return os.WriteFile(filepath.Join(s.artifactDir, name), data, 0o644)
}

func (s *Service) readArtifactLocked(digest string) (SkillArtifact, error) {
	name := strings.TrimPrefix(digest, "sha256:") + ".json"
	data, err := os.ReadFile(filepath.Join(s.artifactDir, name))
	if err != nil {
		return SkillArtifact{}, err
	}
	var artifact SkillArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return SkillArtifact{}, err
	}
	return artifact, nil
}

func (s *Service) audit(actor, action, target, result, message string) {
	s.state.AuditEvents = append(s.state.AuditEvents, AuditEvent{
		ID:        "aud-" + hashString(actor + action + target + time.Now().UTC().Format(time.RFC3339Nano))[:12],
		Actor:     actor,
		Action:    action,
		Target:    target,
		Result:    result,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	})
}

func (s *Service) createDraftLocked(req CreateDraftRequest) (*SkillDraft, error) {
	if err := validateDraftRequest(req); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	draft := &SkillDraft{
		ID:                 idFor(req.Namespace, req.Name, req.Version),
		Namespace:          req.Namespace,
		Name:               req.Name,
		Version:            req.Version,
		Description:        req.Description,
		Visibility:         defaultString(req.Visibility, "project-private"),
		Source:             defaultString(req.Source, "internal"),
		Status:             draftStatusDraft,
		CreatedBy:          defaultString(req.CreatedBy, "fde"),
		RuntimePayload:     normalizePayload(req.RuntimePayload),
		Dependencies:       req.Dependencies,
		PermissionManifest: req.PermissionManifest,
		Assets:             req.Assets,
		Evaluation:         req.Evaluation,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if _, ok := s.state.Drafts[draft.ID]; ok {
		return nil, fmt.Errorf("%w: draft already exists", ErrConflict)
	}
	if _, ok := s.state.Published[draft.ID]; ok {
		return nil, fmt.Errorf("%w: published skill already exists", ErrConflict)
	}
	s.state.Drafts[draft.ID] = draft
	return draft, nil
}

func (s *Service) seed() error {
	req := CreateDraftRequest{
		Namespace:   "common",
		Name:        "echo-template",
		Version:     "1.0.0",
		Description: "Template Skill that renders input text for MVP smoke tests.",
		Visibility:  "company-wide",
		Source:      "internal-seed",
		CreatedBy:   "platform",
		RuntimePayload: RuntimePayload{
			Mode:       "in_process",
			Interface:  "adp.skill.runtime/v1",
			Kind:       "template",
			Entrypoint: "runtime/template",
			Template:   "Processed {{text}}",
		},
		PermissionManifest: PermissionManifest{
			DataDomains: []string{"demo"},
			Filesystem:  FilePermission{Read: []string{"/mnt/input"}, Write: []string{"/tmp/adp-skill"}},
			Kubernetes:  KubernetesAccess{APIAccess: false},
		},
		Assets: SkillAssets{
			Examples: []string{"text=invoice-123"},
		},
		Evaluation: SkillEvaluation{
			Cases: []EvaluationCase{
				{
					Name:             "echo smoke",
					Input:            map[string]string{"text": "invoice-123"},
					ExpectedContains: []string{"Processed", "invoice-123"},
				},
			},
		},
	}
	draft, err := s.createDraftSeed(req)
	if err != nil {
		return err
	}
	result := runEvaluation(draft.RuntimePayload, draft.Evaluation.Cases)
	draft.Evaluation.LastResult = &result
	draft.Status = draftStatusEvaluated
	_, err = s.publishDraftSeed(draft.ID)
	return err
}

func (s *Service) createDraftSeed(req CreateDraftRequest) (*SkillDraft, error) {
	if err := validateDraftRequest(req); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	draft := &SkillDraft{
		ID:                 idFor(req.Namespace, req.Name, req.Version),
		Namespace:          req.Namespace,
		Name:               req.Name,
		Version:            req.Version,
		Description:        req.Description,
		Visibility:         req.Visibility,
		Source:             req.Source,
		Status:             draftStatusDraft,
		CreatedBy:          req.CreatedBy,
		RuntimePayload:     normalizePayload(req.RuntimePayload),
		PermissionManifest: req.PermissionManifest,
		Assets:             req.Assets,
		Evaluation:         req.Evaluation,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	s.state.Drafts[draft.ID] = draft
	return draft, nil
}

func (s *Service) publishDraftSeed(id string) (*PublishedSkill, error) {
	draft := s.state.Drafts[id]
	now := time.Now().UTC()
	sbom := buildSBOM(draft, now)
	provenance := Provenance{
		PredicateType: "https://slsa.dev/provenance/v1",
		Builder:       "agent-skill-registry-mvp",
		Source:        draft.Source,
		BuiltAt:       now,
	}
	published := PublishedSkill{
		ID:                 draft.ID,
		Namespace:          draft.Namespace,
		Name:               draft.Name,
		Version:            draft.Version,
		Description:        draft.Description,
		Visibility:         draft.Visibility,
		Source:             draft.Source,
		Status:             publishedStatus,
		RuntimePayload:     draft.RuntimePayload,
		PermissionManifest: stripSecretValues(draft.PermissionManifest),
		Assets:             draft.Assets,
		EvaluationResult:   *draft.Evaluation.LastResult,
		SBOM:               sbom,
		Provenance:         provenance,
		PublishedAt:        now,
	}
	artifact := SkillArtifact{
		MediaType:          "application/vnd.oci.image.manifest.v1+json",
		ArtifactType:       "application/vnd.z.adp.skill.v1+json",
		SchemaVersion:      "adp.skill.artifact/v1",
		Skill:              published,
		PermissionManifest: published.PermissionManifest,
		SBOM:               sbom,
		Provenance:         provenance,
	}
	digest, err := digestJSON(artifact)
	if err != nil {
		return nil, err
	}
	published.Digest = digest
	published.ArtifactRef = fmt.Sprintf("local-skill-registry/%s/%s@%s", published.Namespace, published.Name, digest)
	published.SignatureKeyID = "z-root-dev"
	published.SignatureScope = "z-global"
	published.Signature = s.sign(digest + ":" + published.SignatureScope)
	artifact.Skill = published
	if err := s.writeArtifactLocked(published.Digest, artifact); err != nil {
		return nil, err
	}
	draft.Status = draftStatusPublished
	s.state.Published[published.ID] = &published
	s.audit("system", "seed.publish", id, "ok", "seed skill")
	return &published, s.saveLocked()
}

func verifyArtifact(skill PublishedSkill, artifact SkillArtifact, s *Service) error {
	artifact.Skill.Digest = ""
	artifact.Skill.ArtifactRef = ""
	artifact.Skill.Signature = ""
	artifact.Skill.SignatureKeyID = ""
	artifact.Skill.SignatureScope = ""
	digest, err := digestJSON(artifact)
	if err != nil {
		return err
	}
	if digest != skill.Digest {
		return fmt.Errorf("%w: artifact digest mismatch", ErrInvalid)
	}
	if !s.verify(skill.Digest+":"+skill.SignatureScope, skill.Signature) {
		return fmt.Errorf("%w: artifact signature mismatch", ErrInvalid)
	}
	return nil
}

func bundleForSigning(bundle OfflineBundle) OfflineBundle {
	copyBundle := bundle
	copyBundle.Signature = ""
	return copyBundle
}

func revocationListForSigning(list SignedRevocationList) SignedRevocationList {
	copyList := list
	copyList.Signature = ""
	return copyList
}
