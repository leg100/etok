package github

import (
	"bytes"
	"encoding/json"
	"strings"
)

// CheckMetadata is info stored in the 'external id' attribute of a check run
type CheckMetadata struct {
	// k8s Namespace
	Namespace string
	// Current etok run id
	Current string
	// Previous etok run id
	Previous string
	// Command: plan or apply
	Command string
	// Etok workspace name
	Workspace string
}

// ToStringPtr serializes the metadata into a JSON string pointer, for
// populating the 'external id' attribute of a check run
func (m *CheckMetadata) ToStringPtr() *string {
	encodedMetadata := new(bytes.Buffer)

	err := json.NewEncoder(encodedMetadata).Encode(m)
	if err != nil {
		panic("unable to encode check run metadata: " + err.Error())
	}

	encodedMetadataStr := encodedMetadata.String()
	return &encodedMetadataStr
}

// newCheckMetadata deserializes a JSON string pointer into a metadata obj
func newCheckMetadata(str *string) CheckMetadata {
	metadata := CheckMetadata{}
	if err := json.NewDecoder(strings.NewReader(*str)).Decode(&metadata); err != nil {
		panic("unable to decode check run metadata: " + err.Error())
	}
	return metadata
}
