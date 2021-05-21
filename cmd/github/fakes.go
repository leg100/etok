package github

import (
	"bytes"
	"context"
	"io"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeInstallsMgr struct {
	checkRunUpdates []*checkRunUpdate
}

func (m *fakeInstallsMgr) send(_ int64, inv invokable) error {
	m.checkRunUpdates = append(m.checkRunUpdates, inv.(*checkRunUpdate))

	return nil
}

type fakeTokenRefresher struct{}

func (tr *fakeTokenRefresher) refreshToken(_ int64) (string, error) {
	return "token123", nil
}

type fakeSender struct {
	u *checkRunUpdate
}

func (s *fakeSender) send(_ int64, inv invokable) error {
	s.u = inv.(*checkRunUpdate)

	return nil
}

type fakeStreamer struct{}

func (s *fakeStreamer) Stream(ctx context.Context, key client.ObjectKey) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBufferString("fake logs")), nil
}
