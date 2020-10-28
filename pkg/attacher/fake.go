package attacher

import (
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func FakeAttach(out io.Writer, cfg rest.Config, pod *corev1.Pod, in *os.File, containerName, magicString string) error {
	out.Write([]byte("fake attach"))
	return nil
}
