package tftmpl

import (
	"fmt"
	"io"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// newTFVarsTmpl writes the hcat services template to a .tfvars file. This is
// used by hcat for and for monitoring service changes from Consul evaluating a
// run condition to trigger a condition.
func newTFVarsTmpl(w io.Writer, filename string, input *RootModuleInputData) error {
	if err := writePreamble(w, input.Task, filename); err != nil {
		return err
	}

	// First append the SourceInput templating
	areServicesAppended := false
	if input.SourceInput != nil {
		if err := input.SourceInput.appendTemplate(w); err != nil {
			return err
		}
		areServicesAppended = input.SourceInput.ServicesAppended()
	}

	// Next append the condition templating
	// SourceInput services take precedence over Condition services, so only append Condition services if services
	// weren't appended by SourceInput
	if input.Condition != nil {
		if input.Condition.ServicesAppended() {
			if !areServicesAppended {
				if err := input.Condition.appendTemplate(w); err != nil {
					return err
				}
				areServicesAppended = input.Condition.ServicesAppended()
			}
			// Else do nothing if services are already appended
		} else {
			if err := input.Condition.appendTemplate(w); err != nil {
				return err
			}
		}
	}

	// Finally, append raw services if no services have been added yet
	hclFile := hclwrite.NewEmptyFile()
	if !areServicesAppended {
		body := hclFile.Body()
		appendRawServiceTemplateValues(body, input.Services)
	}

	_, err := hclFile.WriteTo(w)
	return err
}

// newProvidersTFVars writes input variables for configured Terraform providers.
func newProvidersTFVars(w io.Writer, filename string, input *RootModuleInputData) error {
	err := writePreamble(w, input.Task, filename)
	if err != nil {
		return err
	}

	hclFile := hclwrite.NewEmptyFile()
	body := hclFile.Body()
	body.AppendNewline()

	lastIdx := len(input.Providers) - 1
	for i, p := range input.Providers {
		obj := p.ObjectVal()
		body.SetAttributeValue(p.Name, *obj)
		if i != lastIdx {
			body.AppendNewline()
		}
	}

	_, err = hclFile.WriteTo(w)
	return err
}

// appendRawServiceTemplateValues appends raw lines representing blocks that
// assign value to the services variable `VariableServices` with `hcat` template
// syntax for dynamic rendering of Consul dependency values.
//
// services = {
//   <service>: {
//	   {{ <template syntax> }}
//   },
// }
func appendRawServiceTemplateValues(body *hclwrite.Body, services []Service) {
	tokens := make([]*hclwrite.Token, 0, len(services)+2)
	tokens = append(tokens, &hclwrite.Token{
		Type:  hclsyntax.TokenOBrace,
		Bytes: []byte("{"),
	})

	for _, s := range services {
		rawService := fmt.Sprintf(serviceBaseTmpl, s.hcatQuery())
		token := hclwrite.Token{
			Type:  hclsyntax.TokenNil,
			Bytes: []byte(rawService),
		}
		tokens = append(tokens, &token)
	}

	tokens = append(tokens, &hclwrite.Token{
		Bytes: []byte("\n}"),
	})
	body.AppendNewline()
	body.SetAttributeRaw("services", tokens)
}

// serviceBaseTmpl is the raw template following hcat syntax for addresses of
// Consul services.
const serviceBaseTmpl = `
{{- with $srv := service %s }}
  {{- range $s := $srv}}
  "{{ joinStrings "." .ID .Node .Namespace .NodeDatacenter }}" = {
{{ HCLService $s | indent 4 }}
  },
  {{- end}}
{{- end}}`
