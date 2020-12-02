package v1alpha1

import "time"

const (
	DefaultHandshakeTimeout = 10 * time.Second
)

// AttachSpec defines behavour for clients attaching to the runner TTY
type AttachSpec struct {
	// Toggle whether runner should wait for a handshake from client
	Handshake bool `json:"handshake"`
	// How long to wait for handshake before timing out
	// +kubebuilder:default="10s"
	HandshakeTimeout string `json:"handshakeTimeout"`
}
