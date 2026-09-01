package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
	"github.com/zclconf/go-cty/cty"
)

// StegraProviderVersionOwnershipRule keeps reusable modules free of provider
// version decisions and requires runnable roots to own exact provider versions.
type StegraProviderVersionOwnershipRule struct {
	tflint.DefaultRule
}

func NewStegraProviderVersionOwnershipRule() *StegraProviderVersionOwnershipRule {
	return &StegraProviderVersionOwnershipRule{}
}

func (r *StegraProviderVersionOwnershipRule) Name() string {
	return "stegra_provider_version_ownership"
}

func (r *StegraProviderVersionOwnershipRule) Enabled() bool             { return false }
func (r *StegraProviderVersionOwnershipRule) Severity() tflint.Severity { return tflint.ERROR }
func (r *StegraProviderVersionOwnershipRule) Link() string              { return "" }

type providerVersionOwnershipConfig struct {
	RootDirectories   []string `hclext:"root_directories,optional"`
	ModuleDirectories []string `hclext:"module_directories,optional"`
}

type rootProviderOwnership struct {
	sources         map[string]struct{}
	issueRange      hcl.Range
	hasLocalModules bool
}

type moduleProviderRequirement struct {
	source     string
	modulePath string
}

func (r *StegraProviderVersionOwnershipRule) Check(runner tflint.Runner) error {
	cfg := providerVersionOwnershipConfig{}
	if err := runner.DecodeRuleConfig(r.Name(), &cfg); err != nil {
		return err
	}
	if len(cfg.RootDirectories) == 0 || len(cfg.ModuleDirectories) == 0 {
		return fmt.Errorf("%s: root_directories and module_directories are required", r.Name())
	}

	rootDirectories := cleanDirectories(cfg.RootDirectories)
	moduleDirectories := cleanDirectories(cfg.ModuleDirectories)
	if len(rootDirectories) == 0 || len(moduleDirectories) == 0 {
		return fmt.Errorf("%s: root_directories and module_directories must contain non-empty paths", r.Name())
	}
	files, err := runner.GetFiles()
	if err != nil {
		return err
	}
	rootOwnership := map[string]*rootProviderOwnership{}

	for filename, file := range files {
		if strings.HasSuffix(filename, ".tf.json") || filepath.Ext(filename) == ".json" {
			continue
		}

		isRoot := isUnderAnyDirectory(filename, rootDirectories)
		isModule := isUnderAnyDirectory(filename, moduleDirectories)
		if !isRoot && !isModule {
			continue
		}
		if isRoot && isModule {
			return fmt.Errorf("%s: %s matches both root_directories and module_directories", r.Name(), filename)
		}
		directory := filepath.ToSlash(filepath.Dir(filepath.Clean(filename)))

		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		if isRoot {
			for _, block := range body.Blocks {
				if !isLocalModuleBlock(block) {
					continue
				}
				ownership, exists := rootOwnership[directory]
				if !exists {
					ownership = &rootProviderOwnership{
						sources:    map[string]struct{}{},
						issueRange: block.TypeRange,
					}
					rootOwnership[directory] = ownership
				}
				ownership.hasLocalModules = true
			}
		}
		for _, terraformBlock := range body.Blocks {
			if terraformBlock.Type != "terraform" {
				continue
			}
			for _, requiredProvidersBlock := range terraformBlock.Body.Blocks {
				if requiredProvidersBlock.Type != "required_providers" {
					continue
				}
				if isRoot {
					if ownership, exists := rootOwnership[directory]; !exists {
						rootOwnership[directory] = &rootProviderOwnership{
							sources:    map[string]struct{}{},
							issueRange: requiredProvidersBlock.TypeRange,
						}
					} else {
						ownership.issueRange = requiredProvidersBlock.TypeRange
					}
				}
				for providerName, providerAttribute := range requiredProvidersBlock.Body.Attributes {
					if isRoot {
						if source, hasSource := requiredProviderStringAttribute(providerAttribute.Expr, "source"); hasSource {
							rootOwnership[directory].sources[normalizeProviderSource(source)] = struct{}{}
						}
					}
					versionExpression, hasVersion := requiredProviderVersionExpression(providerAttribute.Expr)
					if isModule && hasVersion {
						if err := runner.EmitIssue(
							r,
							fmt.Sprintf("reusable module provider %q must not declare a version; the root workspace owns provider versions", providerName),
							versionExpression.Range(),
						); err != nil {
							return err
						}
						continue
					}

					if !isRoot {
						continue
					}
					if !hasVersion {
						if err := runner.EmitIssue(
							r,
							fmt.Sprintf("root workspace provider %q must declare an exact version", providerName),
							providerAttribute.NameRange,
						); err != nil {
							return err
						}
						continue
					}

					constraint, diagnostics := versionExpression.Value(nil)
					if diagnostics.HasErrors() || !constraint.IsKnown() || constraint.IsNull() || constraint.Type() != cty.String || !isExactProviderVersion(constraint.AsString()) {
						if err := runner.EmitIssue(
							r,
							fmt.Sprintf("root workspace provider %q must use one exact version, not a range", providerName),
							versionExpression.Range(),
						); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	originalWorkingDirectory, err := runner.GetOriginalwd()
	if err != nil {
		return err
	}
	for rootDirectory, ownership := range rootOwnership {
		if !ownership.hasLocalModules {
			continue
		}
		moduleProviders, err := transitiveModuleProviderRequirements(originalWorkingDirectory, rootDirectory)
		if err != nil {
			return err
		}
		for _, requirement := range moduleProviders {
			if _, exists := ownership.sources[requirement.source]; exists {
				continue
			}
			if err := runner.EmitIssue(
				r,
				fmt.Sprintf(
					"root workspace must declare an exact version for transitive module provider %q (used by %s)",
					requirement.source,
					requirement.modulePath,
				),
				ownership.issueRange,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func requiredProviderVersionExpression(expression hclsyntax.Expression) (hclsyntax.Expression, bool) {
	return requiredProviderObjectAttribute(expression, "version")
}

func requiredProviderObjectAttribute(expression hclsyntax.Expression, attributeName string) (hclsyntax.Expression, bool) {
	object, ok := expression.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return nil, false
	}
	for _, item := range object.Items {
		key, diagnostics := item.KeyExpr.Value(nil)
		if diagnostics.HasErrors() || !key.IsKnown() || key.IsNull() || key.Type() != cty.String || key.AsString() != attributeName {
			continue
		}
		return item.ValueExpr, true
	}
	return nil, false
}

func requiredProviderStringAttribute(expression hclsyntax.Expression, attributeName string) (string, bool) {
	attributeExpression, exists := requiredProviderObjectAttribute(expression, attributeName)
	if !exists {
		return "", false
	}
	value, diagnostics := attributeExpression.Value(nil)
	if diagnostics.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return "", false
	}
	return value.AsString(), true
}

func isExactProviderVersion(constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if strings.HasPrefix(constraint, "=") {
		constraint = strings.TrimSpace(strings.TrimPrefix(constraint, "="))
	}
	if constraint == "" || strings.ContainsAny(constraint, "<>,!~* ") {
		return false
	}
	_, err := version.NewVersion(constraint)
	return err == nil
}

func cleanDirectories(directories []string) []string {
	cleaned := make([]string, 0, len(directories))
	for _, directory := range directories {
		if directory == "" {
			continue
		}
		cleaned = append(cleaned, filepath.ToSlash(filepath.Clean(directory)))
	}
	return cleaned
}

func isUnderAnyDirectory(path string, directories []string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, directory := range directories {
		if isUnderDir(path, directory) {
			return true
		}
	}
	return false
}

func normalizeProviderSource(source string) string {
	return strings.ToLower(strings.TrimPrefix(source, "registry.terraform.io/"))
}

func isLocalModuleBlock(block *hclsyntax.Block) bool {
	if block.Type != "module" {
		return false
	}
	sourceAttribute, exists := block.Body.Attributes["source"]
	if !exists {
		return false
	}
	sourceValue, diagnostics := sourceAttribute.Expr.Value(nil)
	if diagnostics.HasErrors() || !sourceValue.IsKnown() || sourceValue.IsNull() || sourceValue.Type() != cty.String {
		return false
	}
	source := strings.SplitN(sourceValue.AsString(), "?", 2)[0]
	return strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../")
}

func transitiveModuleProviderRequirements(repositoryRoot, rootDirectory string) ([]moduleProviderRequirement, error) {
	repositoryRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return nil, err
	}
	repositoryRoot, err = filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return nil, err
	}
	rootPath := filepath.Join(repositoryRoot, filepath.FromSlash(rootDirectory))
	pending, err := localModulePaths(rootPath, repositoryRoot)
	if err != nil {
		return nil, err
	}

	requirements := map[string]moduleProviderRequirement{}
	visited := map[string]struct{}{}
	for len(pending) > 0 {
		modulePath := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if _, exists := visited[modulePath]; exists {
			continue
		}
		visited[modulePath] = struct{}{}

		relativeModulePath, err := filepath.Rel(repositoryRoot, modulePath)
		if err != nil {
			return nil, err
		}
		providerSources, err := providerSourcesInDirectory(modulePath)
		if err != nil {
			return nil, err
		}
		for _, source := range providerSources {
			normalizedSource := normalizeProviderSource(source)
			if _, exists := requirements[normalizedSource]; !exists {
				requirements[normalizedSource] = moduleProviderRequirement{
					source:     normalizedSource,
					modulePath: filepath.ToSlash(relativeModulePath),
				}
			}
		}

		children, err := localModulePaths(modulePath, repositoryRoot)
		if err != nil {
			return nil, err
		}
		pending = append(pending, children...)
	}

	result := make([]moduleProviderRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		result = append(result, requirement)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].source < result[j].source
	})
	return result, nil
}

func providerSourcesInDirectory(directory string) ([]string, error) {
	sources := []string{}
	err := visitTerraformBodies(directory, func(_ string, body *hclsyntax.Body) error {
		for _, terraformBlock := range body.Blocks {
			if terraformBlock.Type != "terraform" {
				continue
			}
			for _, requiredProvidersBlock := range terraformBlock.Body.Blocks {
				if requiredProvidersBlock.Type != "required_providers" {
					continue
				}
				for _, providerAttribute := range requiredProvidersBlock.Body.Attributes {
					if source, exists := requiredProviderStringAttribute(providerAttribute.Expr, "source"); exists {
						sources = append(sources, source)
					}
				}
			}
		}
		return nil
	})
	return sources, err
}

func localModulePaths(directory, repositoryRoot string) ([]string, error) {
	paths := []string{}
	err := visitTerraformBodies(directory, func(_ string, body *hclsyntax.Body) error {
		for _, moduleBlock := range body.Blocks {
			if moduleBlock.Type != "module" {
				continue
			}
			sourceAttribute, exists := moduleBlock.Body.Attributes["source"]
			if !exists {
				continue
			}
			sourceValue, diagnostics := sourceAttribute.Expr.Value(nil)
			if diagnostics.HasErrors() || !sourceValue.IsKnown() || sourceValue.IsNull() || sourceValue.Type() != cty.String {
				continue
			}
			source := strings.SplitN(sourceValue.AsString(), "?", 2)[0]
			if !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../") {
				continue
			}

			candidate := filepath.Clean(filepath.Join(directory, filepath.FromSlash(source)))
			candidate, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			relativeCandidate, err := filepath.Rel(repositoryRoot, candidate)
			if err != nil || relativeCandidate == ".." || strings.HasPrefix(relativeCandidate, ".."+string(filepath.Separator)) {
				continue
			}
			info, err := os.Stat(candidate)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			if info.IsDir() {
				paths = append(paths, candidate)
			}
		}
		return nil
	})
	return paths, err
}

func visitTerraformBodies(directory string, visit func(string, *hclsyntax.Body) error) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" {
			continue
		}
		filename := filepath.Join(directory, entry.Name())
		source, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		file, diagnostics := hclsyntax.ParseConfig(source, filename, hcl.Pos{Line: 1, Column: 1})
		if diagnostics.HasErrors() {
			return fmt.Errorf("failed to parse %s while checking provider ownership: %s", filename, diagnostics.Error())
		}
		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		if err := visit(filename, body); err != nil {
			return err
		}
	}
	return nil
}
