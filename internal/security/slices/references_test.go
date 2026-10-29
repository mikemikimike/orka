package slices

import (
	"strconv"
	"strings"
	"testing"

	"github.com/orka-agents/orka/internal/security"
	"github.com/orka-agents/orka/internal/store"
)

func TestReferenceFilesFollowInRepositoryGoImports(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n\ngo 1.22\n")
	writeFile(t, root, "main.go", "package main\n\nimport (\n\t\"fmt\"\n\t\"example.com/app/internal/core\"\n)\n\nfunc main() { fmt.Println(core.Run()) }\n")
	writeFile(t, root, "internal/core/core.go", "package core\n\nimport \"example.com/app/apis/types\"\n\nfunc Run() types.Value { return types.Value{} }\n")
	writeFile(t, root, "internal/core/core_test.go", "package core\n")
	writeFile(t, root, "apis/types/types.go", "package types\n\ntype Value struct{}\n")
	writeFile(t, root, "unrelated/other.go", "package unrelated\n")
	writeFile(t, root, "third_party/lib/lib.go", "package lib\n")

	slices, err := MapRepository(root, MapperOptions{RepositoryScan: "repo", BaselineAudit: true})
	if err != nil {
		t.Fatalf("MapRepository() error = %v", err)
	}
	refs, err := ReferenceFiles(root, slices)
	if err != nil {
		t.Fatalf("ReferenceFiles() error = %v", err)
	}

	core := sliceByTitle(t, slices, "Go package internal/core")
	got := referenceSummary(refs[core.ID])
	for _, want := range []string{
		"internal/core/core.go=slice:5",
		"internal/core/core_test.go=slice:1",
		"apis/types/types.go=imports:3",
		"main.go=imported-by:8",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("core references = %s, want %s", got, want)
		}
	}
	if strings.Contains(got, "unrelated/") || strings.Contains(got, "third_party/") {
		t.Fatalf("core references include unrelated or vendored files: %s", got)
	}

	baseline := sliceByTitle(t, slices, "Independent repository audit")
	if baseline.Kind != security.SliceKindBaseline || len(baseline.OwnedFiles) == 0 || baseline.OwnedFiles[0].Path != "main.go" {
		t.Fatalf("baseline slice = %#v, want main.go as its entrypoint", baseline)
	}
	baselineRefs := referenceSummary(refs[baseline.ID])
	if !strings.Contains(baselineRefs, "unrelated/other.go=repository") || strings.Contains(baselineRefs, "third_party/") {
		t.Fatalf("baseline references = %s, want the repository index without vendored code", baselineRefs)
	}
}

func TestMapRepositoryAddsInvestigationSlices(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/flow/a.go", "package flow\n")
	writeFile(t, root, "pkg/flow/a_test.go", "package flow\n")
	writeFile(t, root, "pkg/sink/b.go", "package sink\n")
	writeFile(t, root, "cmd/app/main.go", "package main\n")

	slices, err := MapRepository(root, MapperOptions{RepositoryScan: "repo", Investigations: []security.SecurityInvestigation{
		{Title: "Flow to sink", Question: "Does flow input reach the sink unchecked?", Paths: []string{"pkg/flow", "pkg/sink/b.go", "missing.go"}},
		{Title: "No paths", Question: "Is the entrypoint authenticated?", Paths: []string{"missing"}},
	}})
	if err != nil {
		t.Fatalf("MapRepository() error = %v", err)
	}
	flow := sliceByTitle(t, slices, "Investigation: Flow to sink")
	if flow.Kind != security.SliceKindInvestigation || flow.Summary != "Does flow input reach the sink unchecked?" ||
		!strings.HasPrefix(flow.ID, "slice_inv_") {
		t.Fatalf("investigation slice = %#v", flow)
	}
	if got := filePaths(flow.OwnedFiles); got != "pkg/flow/a.go,pkg/sink/b.go" {
		t.Fatalf("investigation files = %s, want test files skipped and directory expanded", got)
	}
	if got := filePaths(sliceByTitle(t, slices, "Investigation: No paths").OwnedFiles); got != "cmd/app/main.go" {
		t.Fatalf("pathless investigation files = %s, want repository entrypoint", got)
	}
}

func sliceByTitle(t *testing.T, slices []store.ReviewSlice, title string) store.ReviewSlice {
	t.Helper()
	for _, slice := range slices {
		if slice.Title == title {
			return slice
		}
	}
	t.Fatalf("no slice titled %q", title)
	return store.ReviewSlice{}
}

func referenceSummary(refs []security.ReviewContextReferenceFile) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, ref.Path+"="+ref.Relation+":"+strconv.Itoa(ref.Lines))
	}
	return strings.Join(parts, " ")
}

func filePaths(files []store.ReviewSliceFile) string {
	parts := make([]string, 0, len(files))
	for _, file := range files {
		parts = append(parts, file.Path)
	}
	return strings.Join(parts, ",")
}

func TestMapRepositorySkipsBuildOutputOnlyAtTheRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/target/matcher.go", "package target\n")
	writeFile(t, root, "cmd/build/main.go", "package main\n")
	writeFile(t, root, "target/debug/gen.go", "package gen\n")
	writeFile(t, root, "build/out.go", "package out\n")
	slices, err := MapRepository(root, MapperOptions{RepositoryScan: "repo"})
	if err != nil {
		t.Fatalf("MapRepository() error = %v", err)
	}
	owned := map[string]bool{}
	for _, slice := range slices {
		for _, file := range slice.OwnedFiles {
			owned[file.Path] = true
		}
	}
	if !owned["pkg/target/matcher.go"] || !owned["cmd/build/main.go"] {
		t.Fatalf("owned = %v, want nested target and build source reviewed", owned)
	}
	if owned["target/debug/gen.go"] || owned["build/out.go"] {
		t.Fatalf("owned = %v, want root build output skipped", owned)
	}
}

func TestGoImportGraphParsesOnlyIndexedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/app\n")
	writeFile(t, root, "a/a.go", "package a\n\nimport _ \"example.com/app/b\"\n")
	writeFile(t, root, "b/b.go", "package b\n")
	files, err := collectFiles(root)
	if err != nil {
		t.Fatalf("collectFiles() error = %v", err)
	}
	lines := referenceLineCounts(root, files)
	if got := newGoImportGraph(root, files, lines).imports("a"); len(got) != 1 || got[0] != "b" {
		t.Fatalf("imports(a) = %v, want [b]", got)
	}
	// A file over the reference size bound has no line count and is not read.
	delete(lines, "a/a.go")
	if got := newGoImportGraph(root, files, lines).imports("a"); len(got) != 0 {
		t.Fatalf("imports(a) = %v, want none for an unindexed file", got)
	}
}
