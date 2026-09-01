package rules

import (
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
