package controllers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"

	corev1 "k8s.io/api/core/v1"
)

type state struct {
	Serial  int
	Outputs map[string]output
}

type output struct {
	Type  string
	Value string
}

// Unmarshal state from secret
func readState(ctx context.Context, secret *corev1.Secret) (*state, error) {
	data, ok := secret.Data["tfstate"]
	if !ok {
		return nil, errors.New("Expected key tfstate not found in state secret")
	}

	// Return a gzip reader that decompresses on the fly
	gr, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	// Unmarshal state file
	var s state
	if err := json.NewDecoder(gr).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}
