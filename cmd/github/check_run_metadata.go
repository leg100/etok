package github

import (
	"bytes"
	"encoding/json"
	"strings"
)

// CheckRunMetadata is info stored in the 'external id' attribute of a check run
type CheckRunMetadata struct {
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
	// Ordinal number of run
	Iteration int
}

// ToStringPtr serializes the metadata into a JSON string pointer, for
// populating the 'external id' attribute of a check run
func (m *CheckRunMetadata) ToStringPtr() *string {
	encodedMetadata := new(bytes.Buffer)

	err := json.NewEncoder(encodedMetadata).Encode(m)
	if err != nil {
		panic("unable to encode check run metadata: " + err.Error())
	}

	encodedMetadataStr := encodedMetadata.String()
	return &encodedMetadataStr
}

// newCheckRunMetadata deserializes a JSON string pointer into a metadata obj
func newCheckRunMetadata(str *string) CheckRunMetadata {
	metadata := CheckRunMetadata{}
	if err := json.NewDecoder(strings.NewReader(*str)).Decode(&metadata); err != nil {
		panic("unable to decode check run metadata: " + err.Error())
	}
	return metadata
}
