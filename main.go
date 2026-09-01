package main

import (
	"github.com/stegraab/tflint-ruleset-stegra/rules"
	"github.com/terraform-linters/tflint-plugin-sdk/plugin"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		RuleSet: &tflint.BuiltinRuleSet{
			Name:    "stegra",
			Version: "0.1.0",
			Rules: []tflint.Rule{
				rules.NewStegraNewlineAfterKeywordsRule(),
				rules.NewStegraDependsOnLastRule(),
				rules.NewStegraProviderConfigurationLocationsRule(),
				rules.NewStegraNoTypeInNameRule(),
				rules.NewStegraNoMultipleBlankLinesRule(),
				rules.NewStegraNoLeadingTrailingBlankLinesRule(),
				rules.NewStegraNoBlockEdgeBlankLinesRule(),
				rules.NewStegraKeywordsFirstRule(),
				rules.NewStegraBlankLineBetweenBlocksRule(),
				rules.NewStegraNoThisResourceNameRule(),
				rules.NewStegraEmptyBlockOneLineRule(),
				rules.NewStegraNoBlankLinesInRequiredProvidersRule(),
				rules.NewStegraProviderVersionOwnershipRule(),
			},
		},
	})
}
