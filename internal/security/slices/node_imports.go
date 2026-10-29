package slices

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const maxPackageJSONBytes = 1 << 20

var (
	nodeSourceExtensions = []string{extTS, extTSX, extJS, extJSX, extMJS, extCJS}
	// nodeImportPattern finds module specifiers in import and export
	// statements, dynamic imports, and require calls.
	nodeImportPattern = regexp.MustCompile(`(?:\bfrom|\bimport|\brequire)\s*\(?\s*["']([^"'\n]+)["']`)
)

// nodeImportGraph links TypeScript and JavaScript files through relative
// imports and imports of workspace packages in the checkout. Test files do
// not count as importers.
type nodeImportGraph struct {
	edges   map[string]map[string]struct{}
	reverse map[string]map[string]struct{}
}

type nodePackage struct {
	name string
	dir  string
	main string
}

// Only files with a line count, which are regular and within the reference
// size bound, are read.
func newNodeImportGraph(root string, files []repoFile, lines map[string]int) *nodeImportGraph {
	graph := &nodeImportGraph{edges: map[string]map[string]struct{}{}, reverse: map[string]map[string]struct{}{}}
	known := make(map[string]struct{}, len(files))
	var packages []nodePackage
	for _, file := range files {
		known[file.Path] = struct{}{}
		if file.Name == packageJSONFile {
			if pkg, ok := readNodePackage(filepath.Join(root, filepath.FromSlash(file.Path)), file.Dir); ok {
				packages = append(packages, pkg)
			}
		}
	}
	for _, file := range files {
		if _, ok := lines[file.Path]; !ok || !isNodeSource(file.Path) || isNodeTestFile(file.Path) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			continue
		}
		for _, match := range nodeImportPattern.FindAllSubmatch(data, -1) {
			target := resolveNodeImport(file.Dir, string(match[1]), known, packages)
			if target == "" || target == file.Path {
				continue
			}
			link(graph.edges, file.Path, target)
			link(graph.reverse, target, file.Path)
		}
	}
	return graph
}

func (g *nodeImportGraph) imports(file string) []string   { return sortedKeys(g.edges[file]) }
func (g *nodeImportGraph) importers(file string) []string { return sortedKeys(g.reverse[file]) }

func resolveNodeImport(fromDir, spec string, known map[string]struct{}, packages []nodePackage) string {
	var base string
	switch {
	case strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../"):
		base = path.Join(fromDir, spec)
	default:
		for _, pkg := range packages {
			if spec == pkg.name && pkg.main != "" {
				if _, ok := known[pkg.main]; ok {
					return pkg.main
				}
			}
			if spec == pkg.name || strings.HasPrefix(spec, pkg.name+"/") {
				base = path.Join(pkg.dir, strings.TrimPrefix(spec, pkg.name))
				break
			}
		}
	}
	if base == "" || strings.HasPrefix(base, "../") {
		return ""
	}
	candidates := []string{base}
	// TypeScript ESM imports name the emitted JavaScript file.
	stem := base
	switch path.Ext(base) {
	case extJS, extJSX, extMJS, extCJS:
		stem = strings.TrimSuffix(base, path.Ext(base))
	}
	for _, ext := range nodeSourceExtensions {
		candidates = append(candidates, stem+ext)
	}
	for _, ext := range nodeSourceExtensions {
		candidates = append(candidates, path.Join(base, "index"+ext), path.Join(base, "src", "index"+ext))
	}
	for _, candidate := range candidates {
		if _, ok := known[candidate]; ok && isNodeSource(candidate) {
			return candidate
		}
	}
	return ""
}

func readNodePackage(fullPath, dir string) (nodePackage, bool) {
	info, err := os.Lstat(fullPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPackageJSONBytes {
		return nodePackage{}, false
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nodePackage{}, false
	}
	var manifest struct {
		Name string `json:"name"`
		Main string `json:"main"`
	}
	if json.Unmarshal(data, &manifest) != nil || strings.TrimSpace(manifest.Name) == "" {
		return nodePackage{}, false
	}
	pkg := nodePackage{name: strings.TrimSpace(manifest.Name), dir: dir}
	if manifest.Main != "" {
		pkg.main = path.Join(dir, manifest.Main)
	}
	return pkg, true
}

func isNodeSource(filePath string) bool {
	return slices.Contains(nodeSourceExtensions, path.Ext(filePath))
}

func isNodeTestFile(filePath string) bool {
	base := path.Base(filePath)
	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
}
