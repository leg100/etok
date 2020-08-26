// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/leg100/stok/api/command"
)

//go:generate go run generate.go

func main() {
	for _, c := range command.CommandKinds {
		// create the file
		f, err := os.Create(fmt.Sprintf("v1alpha1/%s_types.go", strings.ReplaceAll(command.CommandKindToCLI(c), "-", "_")))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		// replace <backtick> with `
		contents := backtickReplacer.Replace(schemas)

		// parse and render template
		template.Must(template.New("").Parse(contents)).Execute(f, struct {
			ResourceType string
			Kind         string
		}{
			ResourceType: command.CommandKindToType(c),
			Kind:         c,
		})
	}
}

var backtickReplacer = strings.NewReplacer("<backtick>", "`")

const schemas = `// Code generated by go generate; DO NOT EDIT.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Workspace",type="string",JSONPath=".spec.workspace",description="The workspace of the command"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="The command phase"

// {{ .Kind }} is the Schema for the {{ .ResourceType }} API
type {{ .Kind }} struct {
	metav1.TypeMeta   <backtick>json:",inline"<backtick>
	metav1.ObjectMeta <backtick>json:"metadata,omitempty"<backtick>

	CommandSpec   <backtick>json:"spec,omitempty"<backtick>
	CommandStatus <backtick>json:"status,omitempty"<backtick>
}

// +kubebuilder:object:root=true

// {{ .Kind }}List contains a list of {{ .Kind }}
type {{ .Kind }}List struct {
	metav1.TypeMeta <backtick>json:",inline"<backtick>
	metav1.ListMeta <backtick>json:"metadata,omitempty"<backtick>
	Items           []{{ .Kind }} <backtick>json:"items"<backtick>
}

func init() {
	SchemeBuilder.Register(&{{ .Kind }}{}, &{{ .Kind }}List{})
}
`
