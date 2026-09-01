package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/terraform-linters/tflint-plugin-sdk/helper"
)

func TestStegraProviderVersionOwnershipRule(t *testing.T) {
	rule := NewStegraProviderVersionOwnershipRule()
	config := `
rule "stegra_provider_version_ownership" {
  enabled            = true
  root_directories   = ["environments"]
  module_directories = ["modules"]
}
`

	tests := []struct {
		name     string
		filename string
		content  string
		messages []string
	}{
		{
			name:     "module declares only source",
			filename: "modules/aws/network/provider.tf",
			content: `
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}
`,
		},
		{
			name:     "module declares a version",
			filename: "modules/aws/network/provider.tf",
			content: `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.51.0"
    }
  }
}
`,
			messages: []string{`reusable module provider "aws" must not declare a version; the root workspace owns provider versions`},
		},
		{
			name:     "root declares exact version",
			filename: "environments/prod/eu-north-1/network/provider.tf",
			content: `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "= 6.51.0"
    }
  }
}
`,
		},
		{
			name:     "root declares bare exact version",
			filename: "environments/prod/eu-north-1/network/provider.tf",
			content: `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.51.0"
    }
  }
}
`,
		},
		{
			name:     "root omits version",
			filename: "environments/prod/eu-north-1/network/provider.tf",
			content: `
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}
`,
			messages: []string{`root workspace provider "aws" must declare an exact version`},
		},
		{
			name:     "root declares a range",
			filename: "environments/prod/eu-north-1/network/provider.tf",
			content: `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 6.0.0"
    }
  }
}
`,
			messages: []string{`root workspace provider "aws" must use one exact version, not a range`},
		},
		{
			name:     "unconfigured directory is ignored",
			filename: "examples/network/provider.tf",
			content: `
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := helper.TestRunner(t, map[string]string{
				test.filename: test.content,
				".tflint.hcl": config,
			})
			if err := rule.Check(runner); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			expected := make(helper.Issues, 0, len(test.messages))
			for _, message := range test.messages {
				expected = append(expected, &helper.Issue{Rule: rule, Message: message})
			}
			helper.AssertIssuesWithoutRange(t, expected, runner.Issues)
		})
	}
}

func TestStegraProviderVersionOwnershipRuleRequiresDirectories(t *testing.T) {
	rule := NewStegraProviderVersionOwnershipRule()
	runner := helper.TestRunner(t, map[string]string{
		"main.tf": `terraform {}`,
	})
	if err := rule.Check(runner); err == nil {
		t.Fatal("expected missing directory configuration to return an error")
	}
}

func TestTransitiveModuleProviderRequirements(t *testing.T) {
	repositoryRoot := t.TempDir()
	rootDirectory := filepath.Join(repositoryRoot, "environments", "prod", "network")
	parentModule := filepath.Join(repositoryRoot, "modules", "parent")
	childModule := filepath.Join(repositoryRoot, "modules", "child")
	for _, directory := range []string{rootDirectory, parentModule, childModule} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeTerraformTestFile(t, filepath.Join(rootDirectory, "main.tf"), `
module "parent" {
  source = "../../../modules/parent"
}
`)
	writeTerraformTestFile(t, filepath.Join(parentModule, "provider.tf"), `
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}

module "child" {
  source = "../child"
}
`)
	writeTerraformTestFile(t, filepath.Join(childModule, "provider.tf"), `
terraform {
  required_providers {
    random = {
      source = "registry.terraform.io/hashicorp/random"
    }
  }
}

module "parent" {
  source = "../parent"
}
`)

	requirements, err := transitiveModuleProviderRequirements(repositoryRoot, "environments/prod/network")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(requirements) != 2 {
		t.Fatalf("expected two transitive providers, got %#v", requirements)
	}
	if requirements[0].source != "hashicorp/aws" || requirements[0].modulePath != "modules/parent" {
		t.Fatalf("unexpected first requirement: %#v", requirements[0])
	}
	if requirements[1].source != "hashicorp/random" || requirements[1].modulePath != "modules/child" {
		t.Fatalf("unexpected second requirement: %#v", requirements[1])
	}
}

func writeTerraformTestFile(t *testing.T, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
