package security

import (
	"strings"
	"testing"

	corev1alpha1 "github.com/orka-agents/orka/api/v1alpha1"
	"github.com/orka-agents/orka/internal/store"
)

func TestThreatModelInvestigationsRoundTripThroughMarkdown(t *testing.T) {
	items := []SecurityInvestigation{
		{Title: "Caller identity", Question: "Can a cached value replace the caller identity a backend receives?", Paths: []string{"pkg/cache/", "../escape", "pkg/auth/token.go"}},
		{Question: "Do generated routes apply the handler checks?"},
		{Title: "duplicate", Question: "do generated routes apply the handler checks?"},
		{Title: "empty"},
	}
	content := RenderThreatModelInvestigations("# Threat Model\n\nBody.\n", items)
	if !strings.HasPrefix(content, "# Threat Model\n\nBody.") || !strings.Contains(content, threatModelInvestigationsHeading) {
		t.Fatalf("rendered threat model = %q", content)
	}

	got := ThreatModelInvestigations(content)
	if len(got) != 2 {
		t.Fatalf("investigations = %#v, want the two valid, distinct questions", got)
	}
	if strings.Join(got[0].Paths, ",") != "pkg/cache,pkg/auth/token.go" {
		t.Fatalf("paths = %v, want unsafe path dropped and directory trimmed", got[0].Paths)
	}
	if got[1].Title != "Do generated routes apply the handler checks?" {
		t.Fatalf("title = %q, want question as fallback title", got[1].Title)
	}

	// Re-rendering replaces the section instead of appending a second one.
	again := RenderThreatModelInvestigations(content, got[:1])
	if strings.Count(again, threatModelInvestigationsMarker) != 1 || len(ThreatModelInvestigations(again)) != 1 {
		t.Fatalf("re-rendered threat model kept stale investigations:\n%s", again)
	}
	if cleared := RenderThreatModelInvestigations(content, nil); strings.Contains(cleared, threatModelInvestigationsMarker) || !strings.Contains(cleared, "Body.") {
		t.Fatalf("cleared threat model = %q", cleared)
	}
	if ThreatModelInvestigations("# Threat Model\n"+threatModelInvestigationsMarker+"\n```json\nnot json\n```\n") != nil {
		t.Fatal("malformed investigations block was accepted")
	}
}

func TestParseThreatModelResultRendersInvestigations(t *testing.T) {
	binding := testResultBinding()
	result := ThreatModelResultEnvelope{
		SchemaVersion:  AgentResultSchemaVersion,
		Kind:           AgentResultKindThreatModel,
		RepositoryScan: binding.RepositoryScan,
		ScanID:         binding.ScanID,
		PolicyDigest:   binding.PolicyDigest,
		ThreatModel:    "# Threat Model\n\nBody.",
		Investigations: []SecurityInvestigation{{Title: "Flow", Question: "Does input reach the sink?", Paths: []string{"cmd"}}},
	}
	content, err := ParseThreatModelResult(mustMarshalSecurityResult(t, result), AgentResultBinding{
		RepositoryScan: binding.RepositoryScan, ScanID: binding.ScanID, PolicyDigest: binding.PolicyDigest,
	})
	if err != nil {
		t.Fatalf("ParseThreatModelResult() error = %v", err)
	}
	got := ThreatModelInvestigations(content)
	if len(got) != 1 || got[0].Question != "Does input reach the sink?" {
		t.Fatalf("investigations = %#v", got)
	}
}

func TestValidateFindingsV2AcceptsReferenceFileEvidence(t *testing.T) {
	manifest := ReviewContextManifest{
		SchemaVersion: SchemaVersionReviewContext,
		SliceID:       "slice_app",
		IncludedFiles: []ReviewContextIncludedFile{{
			Path:               "app.go",
			IncludedLineRanges: []ReviewContextLineRange{{StartLine: 1, EndLine: 2}},
			Readable:           true,
		}},
		ReferenceFiles: []ReviewContextReferenceFile{
			{Path: "app.go", Lines: 40, Relation: ReferenceRelationSlice},
			{Path: "pkg/consumer/consumer.go", Lines: 10, Relation: ReferenceRelationImportedBy},
		},
	}
	validate := func(ref FindingsV2EvidenceRef) FindingValidationResult {
		finding := validFinding()
		finding.Evidence = append(finding.Evidence, ref)
		return ValidateFindingsV2(FindingsV2Artifact{
			SchemaVersion: SchemaVersionFindingsV2,
			Scan:          FindingsV2Scan{Mode: "initial", SliceID: "slice_app"},
			Findings:      []FindingsV2Finding{finding},
		}, manifest, FindingValidationOptions{Namespace: "default", RepositoryScan: "repo", ScanRunID: "scan1", TaskName: "task1"})
	}

	accepted := validate(FindingsV2EvidenceRef{Path: "pkg/consumer/consumer.go", StartLine: 3, EndLine: 9, Role: " Sink "})
	if len(accepted.Accepted) != 1 {
		t.Fatalf("reference evidence dropped: %#v", accepted.Dropped)
	}
	var role string
	for _, ref := range accepted.Accepted[0].Evidence {
		if ref.Path == "pkg/consumer/consumer.go" {
			role = ref.Role
		}
	}
	if role != "sink" {
		t.Fatalf("evidence role = %q, want normalized sink", role)
	}
	if got := validate(FindingsV2EvidenceRef{Path: "app.go", StartLine: 30, EndLine: 35, Role: "made-up"}); len(got.Accepted) != 1 || got.Accepted[0].Evidence[1].Role != "" {
		t.Fatalf("slice file past its excerpt = %#v, want accepted with unknown role dropped", got)
	}
	for name, ref := range map[string]FindingsV2EvidenceRef{
		"past end of file":  {Path: "pkg/consumer/consumer.go", StartLine: 9, EndLine: 11},
		"unlisted file":     {Path: "pkg/other/other.go", StartLine: 1, EndLine: 2},
		"inverted in range": {Path: "pkg/consumer/consumer.go", StartLine: 5, EndLine: 4},
	} {
		if got := validate(ref); len(got.Accepted) != 0 {
			t.Fatalf("%s: evidence accepted, want dropped", name)
		}
	}
}

func TestParseFindingsResultBoundsCoverageNotes(t *testing.T) {
	scan := FindingsV2Scan{
		ReviewedFiles: []string{"a.go", "a.go", "../secret", "b.go"},
		ResolvedQuestions: []FindingsV2ResolvedQuestion{
			{Question: " Is input authenticated? ", Resolution: "Yes, by middleware."},
			{Resolution: "no question"},
		},
	}
	for range maxReviewedFiles {
		scan.ReviewedFiles = append(scan.ReviewedFiles, "c.go")
	}
	normalizeFindingsCoverage(&scan)
	if strings.Join(scan.ReviewedFiles, ",") != "a.go,b.go,c.go" {
		t.Fatalf("reviewedFiles = %v", scan.ReviewedFiles)
	}
	if len(scan.ResolvedQuestions) != 1 || scan.ResolvedQuestions[0].Question != "Is input authenticated?" {
		t.Fatalf("resolvedQuestions = %#v", scan.ResolvedQuestions)
	}
}

func TestBuildReviewResultPromptExplainsCrossComponentScope(t *testing.T) {
	scan := &corev1alpha1.RepositoryScan{Spec: corev1alpha1.RepositoryScanSpec{RepoURL: "https://github.com/example/project", Branch: "main"}}
	manifest := testReviewContextManifest("slice_api")
	manifest.ReferenceFiles = []ReviewContextReferenceFile{
		{Path: "api/handler.go", Lines: 10, Relation: ReferenceRelationSlice},
		{Path: "store/db.go", Lines: 10, Relation: ReferenceRelationImports},
		{Path: "cmd/server/main.go", Lines: 10, Relation: ReferenceRelationImportedBy},
	}
	slice := store.ReviewSlice{ID: "slice_api", Title: "API", Kind: "package"}
	got := BuildReviewResultPrompt(scan, "initial", "", "", "", slice, testResultBinding(), manifest, FindingsV2Repository{RepoURL: scan.Spec.RepoURL, Branch: "main"})
	for _, want := range []string{"REFERENCE FILES", "packages this slice imports: store", "packages that import this slice: cmd/server", "scan.resolvedQuestions", "root_control"} {
		if !strings.Contains(got, want) {
			t.Fatalf("review prompt missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "INVESTIGATION SLICE") {
		t.Fatal("package slice prompt contains investigation instructions")
	}

	investigation := store.ReviewSlice{ID: "slice_inv", Title: "Investigation: flow", Kind: SliceKindInvestigation, Summary: "Does input reach the sink?"}
	manifest.ReferenceFiles = []ReviewContextReferenceFile{{Path: "a.go", Lines: 1, Relation: ReferenceRelationRepository}}
	got = BuildReviewResultPrompt(scan, "initial", "", "", "", investigation, testResultBinding(), manifest, FindingsV2Repository{RepoURL: scan.Spec.RepoURL, Branch: "main"})
	for _, want := range []string{"INVESTIGATION SLICE", "Does input reach the sink?", "any of the 1 indexed repository"} {
		if !strings.Contains(got, want) {
			t.Fatalf("investigation prompt missing %q:\n%s", want, got)
		}
	}
}

func TestBuildThreatModelResultPromptRequestsInvestigationsForFullScans(t *testing.T) {
	scan := &corev1alpha1.RepositoryScan{Spec: corev1alpha1.RepositoryScanSpec{RepoURL: "https://github.com/example/project", Branch: "main"}}
	if got := BuildThreatModelResultPrompt(scan, "initial", "", "", "", testResultBinding()); !strings.Contains(got, "investigations array") {
		t.Fatalf("full-scan threat model prompt does not request investigations:\n%s", got)
	}
	if got := BuildThreatModelResultPrompt(scan, "incremental", "base", "head", "", testResultBinding()); strings.Contains(got, "investigations array") {
		t.Fatalf("incremental threat model prompt requests investigations:\n%s", got)
	}
}

// Shapes taken from live reviews: resolved questions written as
// {question, answer, evidence}, an extra scan.limitations key, and narrative
// fields returned as structured JSON.
func TestParseFindingsResultToleratesDescriptiveShapeDrift(t *testing.T) {
	binding := testResultBinding()
	repository := FindingsV2Repository{RepoURL: "https://github.com/example/project", Branch: "main"}
	data := []byte(`{"schemaVersion":1,"kind":"orka.security.findings.v1","repositoryScan":"project","scanId":"scan_123","sliceId":"slice_inv","policyDigest":"sha256:policy","contextDigest":"sha256:context",` +
		`"findings":{"schemaVersion":2,"repository":{"repoURL":"https://github.com/example/project","branch":"main","subPath":"","baseSHA":"","headSHA":""},` +
		`"scan":{"mode":"initial","sliceId":"slice_inv","summary":"Traced the flow.","reviewedFiles":["a.go"],` +
		`"resolvedQuestions":[{"question":"Is the key bound?","answer":"Yes.","evidence":[{"path":"a.go","startLine":1}]},"Is it cached?"],"limitations":["No runtime checks."],"confidenceNotes":"extra"},` +
		`"findings":[{"title":"Key replaced","category":"authz","severity":"medium","confidence":"high","triage":"needs-review","evidence":[{"path":"a.go","startLine":1,"endLine":2,"role":"sink"}],` +
		`"summary":"s","rootCause":"r","reproduction":{"steps":["a"]},"remediation":"fix","suggestedAction":"a","whyTestsDoNotAlreadyCoverThis":"w","suggestedRegressionTest":"t","minimumFixScope":"m","exploitability":"high"}]}}`)
	artifact, err := ParseFindingsResult(data, FindingsResultExpectation{Binding: binding, SliceID: "slice_inv", Mode: "initial", Repository: repository})
	if err != nil {
		t.Fatalf("ParseFindingsResult() error = %v", err)
	}
	questions := artifact.Scan.ResolvedQuestions
	if len(questions) != 2 || questions[0].Resolution != "Yes.\nEvidence: [{\"path\":\"a.go\",\"startLine\":1}]" || questions[1].Question != "Is it cached?" {
		t.Fatalf("resolvedQuestions = %#v", questions)
	}
	if len(artifact.Scan.Limitations) != 1 || len(artifact.Findings) != 1 || artifact.Findings[0].Reproduction != `{"steps":["a"]}` {
		t.Fatalf("artifact = %#v", artifact)
	}

	// Identity stays strict.
	mismatched := strings.Replace(string(data), `"mode":"initial"`, `"mode":"incremental"`, 1)
	if _, err := ParseFindingsResult([]byte(mismatched), FindingsResultExpectation{Binding: binding, SliceID: "slice_inv", Mode: "initial", Repository: repository}); err == nil {
		t.Fatal("ParseFindingsResult() accepted a mismatched scan mode")
	}
	unknownEnvelopeKey := strings.Replace(string(data), `"kind":`, `"note":"x","kind":`, 1)
	if _, err := ParseFindingsResult([]byte(unknownEnvelopeKey), FindingsResultExpectation{Binding: binding, SliceID: "slice_inv", Mode: "initial", Repository: repository}); err == nil {
		t.Fatal("ParseFindingsResult() accepted an unknown envelope key")
	}
}
