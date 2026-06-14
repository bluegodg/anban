package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestXiaozhiManagerOpenAPIDetailsStayInsideXiaozhiClient(t *testing.T) {
	serverRoot := mustServerRoot(t)
	allowedDir := filepath.Join(serverRoot, "internal", "xiaozhiclient")
	for _, file := range goProductionFiles(t, serverRoot) {
		if !containsAny(strings.Join(stringLiteralsOf(t, file), "\n"), []string{
			"/api/open/v1",
			"X-API-Token",
			"inject-message",
			"mcp-call",
			"history/messages",
		}) {
			continue
		}
		if !isWithin(file, allowedDir) {
			t.Fatalf("%s contains xiaozhi manager OpenAPI details; route all xiaozhi calls through internal/xiaozhiclient", rel(t, serverRoot, file))
		}
	}
}

func TestChildAPIDoesNotImportStoreOrXiaozhiClient(t *testing.T) {
	serverRoot := mustServerRoot(t)
	childAPIDir := filepath.Join(serverRoot, "internal", "childapi")
	for _, file := range goProductionFiles(t, childAPIDir) {
		for _, importPath := range importsOf(t, file) {
			if strings.Contains(importPath, "/server/internal/store") || strings.Contains(importPath, "/server/internal/xiaozhiclient") {
				t.Fatalf("%s imports %q; childapi must call domains only", rel(t, serverRoot, file), importPath)
			}
		}
	}
}

func TestDomainsDoNotImportEachOther(t *testing.T) {
	serverRoot := mustServerRoot(t)
	domainsDir := filepath.Join(serverRoot, "internal", "domains")
	for _, file := range goProductionFiles(t, domainsDir) {
		for _, importPath := range importsOf(t, file) {
			if strings.Contains(importPath, "/server/internal/domains/") {
				t.Fatalf("%s imports %q; domains must collaborate through pkg/types or childapi orchestration", rel(t, serverRoot, file), importPath)
			}
		}
	}
}

func TestDockerComposeWiresAnbanAllowedOrigins(t *testing.T) {
	serverRoot := mustServerRoot(t)
	repoRoot := filepath.Clean(filepath.Join(serverRoot, ".."))
	composePath := filepath.Join(repoRoot, "docker-compose.yml")
	body, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	if !strings.Contains(string(body), "ANBAN_ALLOWED_ORIGINS:") {
		t.Fatalf("docker-compose.yml anban service must pass ANBAN_ALLOWED_ORIGINS so child web CORS works in Compose deployments")
	}
}

func TestDockerComposeKeepsAnbanOptionalByProfile(t *testing.T) {
	serverRoot := mustServerRoot(t)
	repoRoot := filepath.Clean(filepath.Join(serverRoot, ".."))
	composePath := filepath.Join(repoRoot, "docker-compose.yml")
	body, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}

	xiaozhiBlock := composeServiceBlock(t, string(body), "xiaozhi")
	if strings.Contains(xiaozhiBlock, "depends_on") && strings.Contains(xiaozhiBlock, "anban") {
		t.Fatal("docker-compose.yml xiaozhi service must not depend on anban; original xiaozhi must be deployable alone")
	}

	anbanBlock := composeServiceBlock(t, string(body), "anban")
	if !strings.Contains(anbanBlock, "profiles:") || !strings.Contains(anbanBlock, "anban") {
		t.Fatal("docker-compose.yml anban service must be behind an explicit profile so default compose up keeps anban optional")
	}
}

func TestDockerComposeAnbanBuildContextHasDockerfile(t *testing.T) {
	serverRoot := mustServerRoot(t)
	repoRoot := filepath.Clean(filepath.Join(serverRoot, ".."))
	composePath := filepath.Join(repoRoot, "docker-compose.yml")
	body, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}

	anbanBlock := composeServiceBlock(t, string(body), "anban")
	if !strings.Contains(anbanBlock, "context: ./server") {
		t.Fatal("docker-compose.yml anban service should build from ./server so compose deployments use this repo's backend")
	}

	dockerfilePath := filepath.Join(serverRoot, "Dockerfile")
	dockerfile, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("anban compose build context points at ./server, but server/Dockerfile is missing or unreadable: %v", err)
	}
	content := string(dockerfile)
	for _, required := range []string{"go build", "./cmd/anban", "CGO_ENABLED=0"} {
		if !strings.Contains(content, required) {
			t.Fatalf("server/Dockerfile must contain %q so compose can build the anban backend", required)
		}
	}
}

func TestAnbanDockerBuildContextIgnoresLocalArtifacts(t *testing.T) {
	serverRoot := mustServerRoot(t)
	dockerignorePath := filepath.Join(serverRoot, ".dockerignore")
	body, err := os.ReadFile(dockerignorePath)
	if err != nil {
		t.Fatalf("server/.dockerignore is required because docker-compose builds anban from ./server: %v", err)
	}

	content := string(body)
	for _, required := range []string{".gotmp-go/", ".gocache-go/", "*.db", "anban.db"} {
		if !strings.Contains(content, required) {
			t.Fatalf("server/.dockerignore must ignore %q so compose builds do not copy local caches or demo data", required)
		}
	}
}

func mustServerRoot(t *testing.T) string {
	t.Helper()

	var candidates []string
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, wd)
	}
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Dir(currentFile))
	}

	for _, candidate := range candidates {
		if root, ok := findAncestorContaining(candidate, "go.mod"); ok {
			return root
		}
	}
	t.Fatalf("could not locate server root from candidates: %v", candidates)
	return ""
}

func findAncestorContaining(start, marker string) (string, bool) {
	dir := filepath.Clean(start)
	for {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func composeServiceBlock(t *testing.T, body, serviceName string) string {
	t.Helper()
	lines := strings.Split(body, "\n")
	header := "  " + serviceName + ":"
	start := -1
	for i, line := range lines {
		if strings.TrimRight(line, "\r") == header {
			start = i
			break
		}
	}
	if start == -1 {
		t.Fatalf("docker-compose.yml missing service %q", serviceName)
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			end = i
			break
		}
		if line == "volumes:" {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func goProductionFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".gocache-go", ".gotmp-go", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func importsOf(t *testing.T, file string) []string {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports %s: %v", file, err)
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	return imports
}

func stringLiteralsOf(t *testing.T, file string) []string {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parse file %s: %v", file, err)
	}
	var values []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatalf("unquote string literal in %s: %v", file, err)
		}
		values = append(values, value)
		return true
	})
	return values
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func isWithin(path, root string) bool {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relPath == "." || (!strings.HasPrefix(relPath, ".."+string(filepath.Separator)) && relPath != "..")
}

func rel(t *testing.T, root, path string) string {
	t.Helper()
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return relPath
}
