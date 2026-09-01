package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	version "github.com/hashicorp/go-version"
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

		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, terraformBlock := range body.Blocks {
			if terraformBlock.Type != "terraform" {
				continue
			}
			for _, requiredProvidersBlock := range terraformBlock.Body.Blocks {
				if requiredProvidersBlock.Type != "required_providers" {
					continue
				}
				for providerName, providerAttribute := range requiredProvidersBlock.Body.Attributes {
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

	return nil
}

func requiredProviderVersionExpression(expression hclsyntax.Expression) (hclsyntax.Expression, bool) {
	object, ok := expression.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return nil, false
	}
	for _, item := range object.Items {
		key, diagnostics := item.KeyExpr.Value(nil)
		if diagnostics.HasErrors() || !key.IsKnown() || key.Type() != cty.String || key.AsString() != "version" {
			continue
		}
		return item.ValueExpr, true
	}
	return nil, false
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
