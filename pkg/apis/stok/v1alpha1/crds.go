package v1alpha1

import (
	"strings"

	"github.com/iancoleman/strcase"
	"k8s.io/api/node/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CmdCRD string

func (crd CmdCRD) Plural() string { return strings.ReplaceAll(string(crd), "-", "") + "s" }

func (crd CmdCRD) Kind() string { return strcase.ToCamel(string(crd)) }

func (crd CmdCRD) GroupVersionResource() schema.GroupVersionResource {
	return v1alpha1.SchemeGroupVersion.WithResource(crd.Plural())
}

func (crd CmdCRD) GroupVersionKindList() schema.GroupVersionKind {
	return v1alpha1.SchemeGroupVersion.WithKind(crd.Kind() + "List")
}
