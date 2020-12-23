package e2e

import (
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
)

func step(t test, args []string, batch []expect.Batcher) error {
	exp, errch, err := expect.SpawnWithArgs(args, 60*time.Second, expect.Tee(nopWriteCloser{os.Stdout}))
	if err != nil {
		return err
	}

	_, err = exp.ExpectBatch(batch, 60*time.Second)
	if err != nil {
		return err
	}

	return <-errch
}

func newNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newSecret(name, creds string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		StringData: map[string]string{
			"GOOGLE_CREDENTIALS": creds,
		},
	}
}

type nopWriteCloser struct {
	f *os.File
}

func (n nopWriteCloser) Write(p []byte) (int, error) {
	return n.f.Write(p)
}

func (n nopWriteCloser) Close() error {
	return nil
}
