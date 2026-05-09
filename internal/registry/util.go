package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func validateDraftRequest(req CreateDraftRequest) error {
	if req.Namespace == "" || req.Name == "" || req.Version == "" {
		return fmt.Errorf("%w: namespace, name, and version are required", ErrInvalid)
	}
	if strings.Contains(req.Version, "latest") || strings.ContainsAny(req.Version, "<>") {
		return fmt.Errorf("%w: draft version must be immutable, not a floating range", ErrInvalid)
	}
	for _, secret := range req.PermissionManifest.Secrets {
		if secret.Value != "" {
			return fmt.Errorf("%w: Skill must declare Secret References, not secret values", ErrInvalid)
		}
	}
	for _, asset := range req.Assets.KnowledgeRefs {
		if asset.URI == "" || asset.Version == "" {
			return fmt.Errorf("%w: Knowledge Asset references require uri and version", ErrInvalid)
		}
	}
	return nil
}

func validateCommunityIngestRequest(req CommunityIngestRequest) error {
	if req.SourceURL == "" || req.SourceVersion == "" || req.SourceDigest == "" {
		return fmt.Errorf("%w: community source url, version, and digest are required", ErrInvalid)
	}
	if req.License == "" {
		return fmt.Errorf("%w: community license is required", ErrInvalid)
	}
	if strings.EqualFold(req.License, "unknown") || strings.EqualFold(req.License, "unlicensed") {
		return fmt.Errorf("%w: community license is not approved", ErrInvalid)
	}
	if req.Scan.Status != "" && req.Scan.Status != "pass" {
		return fmt.Errorf("%w: community scan must pass before ingestion", ErrInvalid)
	}
	if req.Scan.CriticalVulnerabilities > 0 {
		return fmt.Errorf("%w: community skill has critical vulnerabilities", ErrInvalid)
	}
	if req.Skill.Namespace == "" || req.Skill.Name == "" || req.Skill.Version == "" {
		return fmt.Errorf("%w: normalized skill identity is required", ErrInvalid)
	}
	return nil
}

func validatePolicy(draft *SkillDraft) error {
	if draft.PermissionManifest.Kubernetes.APIAccess {
		return fmt.Errorf("%w: Kubernetes API access requires manual elevated-risk policy not supported by MVP", ErrInvalid)
	}
	for _, egress := range draft.PermissionManifest.Network.Egress {
		if egress.Target == "0.0.0.0/0" || egress.Target == "*" {
			return fmt.Errorf("%w: wildcard network egress is not allowed", ErrInvalid)
		}
	}
	if draft.Evaluation.LastResult == nil || !draft.Evaluation.LastResult.Passed {
		return fmt.Errorf("%w: evaluation must pass", ErrInvalid)
	}
	return nil
}

func mapPermissionResources(skill *PublishedSkill) PermissionResources {
	pm := skill.PermissionManifest
	resources := PermissionResources{
		ServiceAccount: "skill-" + sanitizeKubernetesName(skill.Namespace+"-"+skill.Name),
		SecurityContext: SecurityContext{
			RunAsNonRoot:             true,
			ReadOnlyRootFilesystem:   true,
			AllowPrivilegeEscalation: false,
		},
	}
	for _, secret := range pm.Secrets {
		resources.SecretProjections = append(resources.SecretProjections, SecretProjection{
			Name:      secret.Name,
			Required:  secret.Required,
			MountPath: "/var/run/adp/secrets/" + sanitizeKubernetesName(secret.Name),
		})
	}
	for _, egress := range pm.Network.Egress {
		resources.NetworkPolicies = append(resources.NetworkPolicies, NetworkPolicyResource{
			Name:   sanitizeKubernetesName(skill.Namespace + "-" + skill.Name + "-" + egress.Name),
			Target: egress.Target,
			Ports:  append([]int(nil), egress.Ports...),
		})
	}
	if pm.Kubernetes.APIAccess {
		resources.RBACRules = append(resources.RBACRules, RBACRule{
			Resource: "pods",
			Verbs:    []string{"get", "list"},
			Reason:   "declared Kubernetes API access",
		})
	}
	if len(pm.Filesystem.Read) > 0 {
		resources.Volumes = append(resources.Volumes, VolumeResource{
			Name:      "skill-read-inputs",
			ReadOnly:  true,
			Paths:     append([]string(nil), pm.Filesystem.Read...),
			MountPath: "/mnt/input",
		})
	}
	if len(pm.Filesystem.Write) > 0 {
		resources.Volumes = append(resources.Volumes, VolumeResource{
			Name:      "skill-write-workspace",
			ReadOnly:  false,
			Paths:     append([]string(nil), pm.Filesystem.Write...),
			MountPath: "/tmp/adp-skill",
		})
	}
	return resources
}

func mountPathForSkill(skill *PublishedSkill) string {
	return "/var/lib/adp/skills/" + skill.Namespace + "/" + skill.Name + "/" + skill.Version
}

func sanitizeKubernetesName(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "skill"
	}
	return out
}

func normalizePayload(payload RuntimePayload) RuntimePayload {
	payload.Mode = defaultString(payload.Mode, "in_process")
	payload.Interface = defaultString(payload.Interface, "adp.skill.runtime/v1")
	payload.Kind = defaultString(payload.Kind, "template")
	payload.Entrypoint = defaultString(payload.Entrypoint, "runtime/template")
	if payload.Template == "" {
		payload.Template = "Processed {{text}}"
	}
	return payload
}

func runEvaluation(payload RuntimePayload, cases []EvaluationCase) EvaluationResult {
	if len(cases) == 0 {
		cases = []EvaluationCase{
			{
				Name:             "default smoke",
				Input:            map[string]string{"text": "smoke"},
				ExpectedContains: []string{"smoke"},
			},
		}
	}
	result := EvaluationResult{
		Passed:      true,
		CaseResults: make([]EvaluationCaseRun, 0, len(cases)),
		RanAt:       time.Now().UTC(),
	}
	for _, testCase := range cases {
		output, err := renderPayload(payload, testCase.Input)
		caseRun := EvaluationCaseRun{Name: testCase.Name, Output: output, Passed: true}
		if err != nil {
			caseRun.Passed = false
			caseRun.Error = err.Error()
		}
		for _, expected := range testCase.ExpectedContains {
			if !strings.Contains(output, expected) {
				caseRun.Passed = false
				caseRun.Error = "output did not contain " + expected
			}
		}
		if !caseRun.Passed {
			result.Passed = false
			result.FailureCount++
		}
		result.CaseResults = append(result.CaseResults, caseRun)
	}
	result.Score = float64(len(cases)-result.FailureCount) / float64(len(cases))
	if result.Score < 1 {
		result.Warnings = append(result.Warnings, "MVP publication requires all smoke cases to pass")
	}
	return result
}

func renderPayload(payload RuntimePayload, input map[string]string) (string, error) {
	switch payload.Kind {
	case "", "template":
		out := payload.Template
		keys := make([]string, 0, len(input))
		for key := range input {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out = strings.ReplaceAll(out, "{{"+key+"}}", input[key])
		}
		if strings.Contains(out, "{{") {
			return out, fmt.Errorf("unresolved template variable")
		}
		return out, nil
	case "echo":
		return input["text"], nil
	default:
		return "", fmt.Errorf("unsupported MVP runtime payload kind %q", payload.Kind)
	}
}

func stripSecretValues(pm PermissionManifest) PermissionManifest {
	for i := range pm.Secrets {
		pm.Secrets[i].Value = ""
	}
	return pm
}

func buildSBOM(draft *SkillDraft, now time.Time) SBOM {
	components := []SBOMComponent{
		{Name: draft.Name, Version: draft.Version, Type: "skill"},
		{Name: "adp.skill.runtime", Version: draft.RuntimePayload.Interface, Type: "runtime"},
	}
	for _, dep := range draft.Dependencies {
		components = append(components, SBOMComponent{Name: dep.ID, Version: dep.Version, Type: "skill-dependency"})
	}
	return SBOM{
		Format:      "CycloneDX-like-MVP",
		Components:  components,
		GeneratedAt: now,
	}
}

func digestJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hashAny(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hashPermission(pm PermissionManifest) string {
	return "sha256:" + hashAny(pm)
}

func idFor(namespace, name, version string) string {
	return namespace + "/" + name + ":" + version
}

func dependencyIDs(deps []SkillDependency) []string {
	items := make([]string, 0, len(deps))
	for _, dep := range deps {
		items = append(items, dep.ID+":"+dep.Version)
	}
	sort.Strings(items)
	return items
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func boolResult(ok bool) string {
	if ok {
		return "ok"
	}
	return "failed"
}

func summarizeMap(input map[string]string) string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := input[key]
		if len(value) > 24 {
			value = value[:24] + "..."
		}
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ",")
}

func summarizeString(value string) string {
	if len(value) > 80 {
		return value[:80] + "..."
	}
	return value
}

func cloneState(state State) State {
	data, _ := json.Marshal(state)
	var copyState State
	_ = json.Unmarshal(data, &copyState)
	return copyState
}

func cloneDraft(draft *SkillDraft) *SkillDraft {
	data, _ := json.Marshal(draft)
	var copyDraft SkillDraft
	_ = json.Unmarshal(data, &copyDraft)
	return &copyDraft
}

func clonePublished(skill *PublishedSkill) *PublishedSkill {
	data, _ := json.Marshal(skill)
	var copySkill PublishedSkill
	_ = json.Unmarshal(data, &copySkill)
	return &copySkill
}

func cloneCommunityIngestion(ingestion *CommunityIngestion) *CommunityIngestion {
	data, _ := json.Marshal(ingestion)
	var copyIngestion CommunityIngestion
	_ = json.Unmarshal(data, &copyIngestion)
	return &copyIngestion
}
