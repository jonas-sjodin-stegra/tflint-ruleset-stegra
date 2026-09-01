# TFLint Ruleset Stegra

This is a custom TFLint ruleset focused on readable, consistent Terraform code, targeted for Stegra Terraform projects. It currently provides:

- `stegra_newline_after_keywords`: Enforces a blank line after configured attributes (e.g., `count`, `for_each`, `source`) only when another item follows in the same block. Auto-fix inserts the missing blank line.
- `stegra_depends_on_last`: Requires `depends_on` to be the last item (attribute or block) in resource/data/module blocks and to have a blank line above when there are prior items. Auto-fix moves `depends_on` to the end and inserts the blank line as needed.
- `stegra_no_type_in_name`: Prevents repeating type tokens from the resource/data type in the name (e.g., `aws_security_group_rule` should not be named `my_security_group_rule`). Allows the token `main` in both type and name.
- `stegra_provider_configuration_locations`: Allows provider configuration blocks only in specified directories.
- `stegra_no_multiple_blank_lines`: Disallows multiple consecutive blank lines between content. Auto-fix removes extras and keeps a single blank line.
- `stegra_no_leading_trailing_blank_lines`: Disallows leading blank lines and trailing blank lines at EOF. Auto-fix removes leading blanks and trims trailing blanks while preserving exactly one final newline.
- `stegra_no_block_edge_blank_lines`: Disallows leading and trailing blank lines inside any HCL block (resource, data, module, provider, nested blocks). Auto-fix removes the interior edge blank lines.
- `stegra_blank_line_between_blocks`: Requires a blank line between any consecutive top-level `resource`/`data`/`module` blocks. If comments appear immediately before the next block, the blank line is inserted before the first comment so the comments remain attached to that block. Auto-fix inserts missing blank lines.
- `stegra_keywords_first`: Ensures configured attributes appear first in the order listed in `keywords` (supports `resource`, `data`, and `module` blocks). Auto-fix reorders items.
- `stegra_no_this_resource_name`: Forbids using the resource name `this`. Auto-fix renames to `main` and updates `<type>.this` traversals in expressions to `<type>.main` (strings/comments are left untouched).
- `stegra_empty_block_one_line`: Enforces that empty blocks use single-line form `{}`. Auto-fix collapses two-line empty blocks.
- `stegra_no_blank_lines_in_required_providers`: Disallows blank lines anywhere inside `terraform` → `required_providers`. Auto-fix removes only the empty lines (keeps comments).
- `stegra_provider_version_ownership`: Requires reusable modules to omit provider versions and runnable roots to declare exact versions for their complete local-module dependency graph, keeping provider-version ownership next to each root lockfile.

## Requirements

- TFLint v0.46+
- Go v1.25 (only needed for local development)

## Installation

Install via GitHub releases (recommended):

1) In your Terraform project, create `.tflint.hcl`:

```hcl
plugin "stegra" {
  enabled = true
  source  = "github.com/stegraab/tflint-ruleset-stegra"
  version = "0.1.0"
}
```

2) Download the plugin binary referenced by your version:

```
tflint --init
```

3) Run TFLint:

```
tflint
```

## Rules

|Name|Description|Severity|Enabled|Auto-fix|
| --- | --- | --- | --- | --- |
|stegra_newline_after_keywords|Enforces a blank line after selected attributes when followed by more items|ERROR|✔|Insert blank line|
|stegra_depends_on_last|Requires depends_on last with a blank line above when needed|ERROR|✔|Move depends_on to end + insert|
|stegra_no_type_in_name|Prevents repeating type tokens in resource/data names (allows token `main`)|ERROR|✔|N/A|
|stegra_provider_configuration_locations|Allows provider blocks only in specified directories|ERROR|✔|N/A|
|stegra_no_multiple_blank_lines|Disallows multiple consecutive blank lines between content|ERROR|✔|Remove extras (collapse to one)|
|stegra_no_leading_trailing_blank_lines|Disallows leading/trailing blank lines|ERROR|✔|Remove leading/trailing; keep 1 EOF newline|
|stegra_no_block_edge_blank_lines|Disallows leading/trailing blank lines inside any block|ERROR|✔|Remove interior edge blanks|
|stegra_blank_line_between_blocks|Requires a blank line between top-level resource/data/module blocks|ERROR|✔|Insert blank line|
|stegra_keywords_first|Configured attributes must appear first in the order listed|ERROR|✔|Reorder items|
|stegra_no_this_resource_name|Forbids resource name `this`|ERROR|✔|Rename to `main` + update expression refs|
|stegra_empty_block_one_line|Enforces single-line `{}` for empty blocks|ERROR|✔|Collapse to `{}`|
|stegra_no_blank_lines_in_required_providers|Disallows blank lines anywhere in required_providers|ERROR|✔|Remove blank lines|
|stegra_provider_version_ownership|Keeps module providers versionless and requires exact transitive versions in roots|ERROR|Configurable|N/A|

## Auto-fix Examples

These rules support tflint's auto-fixer. Run with:

```
tflint --fix
```

- stegra_no_multiple_blank_lines
  - Bad:
    ```hcl
    resource "aws_vpc" "a" {}


    resource "aws_vpc" "b" {}
    ```
  - Fixed:
    ```hcl
    resource "aws_vpc" "a" {}

    resource "aws_vpc" "b" {}
    ```

- stegra_no_leading_trailing_blank_lines
  - Leading blank line (bad):
    ```hcl

    resource "aws_vpc" "a" {}
    ```
  - Fixed:
    ```hcl
    resource "aws_vpc" "a" {}
    ```
  - Trailing blanks (bad):
    ```hcl
    resource "aws_vpc" "a" {}


    ```
  - Fixed (keeps a single EOF newline):
    ```hcl
    resource "aws_vpc" "a" {}
    ```

- stegra_no_block_edge_blank_lines
  - Bad:
    ```hcl
    module "vpc" {

      source = "./vpc"

    }
    ```
  - Fixed:
    ```hcl
    module "vpc" {
      source = "./vpc"
    }
    ```

- stegra_blank_line_between_blocks
  - Bad:
    ```hcl
    resource "null_resource" "a" {}
    resource "null_resource" "b" {}
    ```
  - Fixed:
    ```hcl
    resource "null_resource" "a" {}

    resource "null_resource" "b" {}
    ```
  - With comments: blank line is inserted before the comment so comments stay attached to the next block
    ```hcl
    resource "null_resource" "a" {}
    # comment about next block
    resource "null_resource" "b" {}
    ```
    Fixed to:
    ```hcl
    resource "null_resource" "a" {}

    # comment about next block
    resource "null_resource" "b" {}
    ```
  - Note: Applies only to top-level blocks, not nested blocks inside a resource/module

- stegra_keywords_first
  - Bad (non-target before targets):
    ```hcl
    resource "aws_vpc" "a" {
      name     = "a"
      for_each = []
    }
    ```
  - Fixed (targets first in configured order):
    ```hcl
    resource "aws_vpc" "a" {
      for_each = []
      name     = "a"
    }
    ```

- stegra_newline_after_keywords (only if more items follow)
  - Bad (keyword not followed by a blank line and more items follow):
    ```hcl
    module "mod" {
      source  = "./module"
      name = "value"
    }
    ```
  - Fixed:
    ```hcl
    module "mod" {
      source  = "./module"

      name = "value"
    }
    ```
  - No issue when keyword is last:
    ```hcl
    module "mod" {
      source = "./module"
    }
    ```
  - Exception in module blocks: `source` followed immediately by `version` does not require a blank line
    ```hcl
    module "mod" {
      source  = "./module"
      version = "~> 1.0"
    }
    ```

- stegra_no_this_resource_name
  - Bad:
    ```hcl
    resource "aws_s3_bucket" "this" {}
    resource "aws_iam_role" "r" {
      name = aws_s3_bucket.this.id
    }
    ```
  - Fixed (resource renamed and expression reference updated):
    ```hcl
    resource "aws_s3_bucket" "main" {}
    resource "aws_iam_role" "r" {
      name = aws_s3_bucket.main.id
    }
    ```
  - Note: References are updated only in expressions (not in plain strings/comments).

- stegra_no_blank_lines_in_required_providers
  - Bad:
    ```hcl
    terraform {
      required_providers {

        aws = {
          source  = "hashicorp/aws"
          version = "6.20.0"
        }

        gitlab = {
          source  = "gitlabhq/gitlab"
          version = "18.5.0"
        }


      }
    }
    ```
  - Fixed (blank line removed, comments would be preserved):
    ```hcl
    terraform {
      required_providers {
        aws = {
          source  = "hashicorp/aws"
          version = "6.20.0"
        }
        gitlab = {
          source  = "gitlabhq/gitlab"
          version = "18.5.0"
        }
      }
    }
    ```

- stegra_empty_block_one_line
  - Bad:
    ```hcl
    resource "random_uuid" "x" {
    }
    ```
  - Fixed:
    ```hcl
    resource "random_uuid" "x" {}
    ```

## Configuration

You must configure some rules using `.tflint.hcl` rule blocks.

- stegra_newline_after_keywords
  - Required option: `keywords` (list of strings)
  - If `keywords` is not set, the rule returns an error.
  - Example:

```hcl
rule "stegra_newline_after_keywords" {
  enabled  = false
  keywords = ["for_each", "count", "source"]
}
```

- stegra_provider_configuration_locations
  - Required option: `allowed_directories` (list of path prefixes relative to repo root)
  - Example:

```hcl
rule "stegra_provider_configuration_locations" {
  enabled             = true
  allowed_directories = ["environments"]
}
```

- stegra_provider_version_ownership
  - Required options: `root_directories` and `module_directories` (path prefixes relative to the repository root)
  - Runnable roots must use one exact provider version, while reusable modules must omit the `version` attribute.
  - The rule follows nested local module sources (`./` and `../`) and requires the root to declare every provider used transitively by those modules.
  - Lockfile selection is outside TFLint's configuration model, so a separate CI check is still needed to verify that `.terraform.lock.hcl` selects the declared versions.
  - Example:

```hcl
rule "stegra_provider_version_ownership" {
  enabled            = true
  root_directories   = ["environments"]
  module_directories = ["modules"]
}
```

Failing reusable module:

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.51.0"
    }
  }
}
```

Passing reusable module and root workspace:

```hcl
# modules/example/provider.tf
terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}

# environments/prod/example/provider.tf
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.51.0"
    }
  }
}
```

- stegra_keywords_first
  - Required option: `keywords` (ordered list of attribute names to appear first)
  - The rule enforces the exact order listed in `keywords` when those attributes are present in a block
  - Applies to `resource`, `data`, and `module` blocks
  - Example:

```hcl
rule "stegra_keywords_first" {
  enabled  = true
  keywords = ["provider", "for_each", "count", "source"]
}
```

## Development

- Run tests
  ```
  make test
  ```

- Build the plugin
  ```
  make build
  ```

- Install locally into `~/.tflint.d/plugins/`
  ```
  make install
  ```

Then use `.tflint.hcl` as shown in Installation and run `tflint`. The Makefile uses a local `GOCACHE` for tests to work in restricted environments.
