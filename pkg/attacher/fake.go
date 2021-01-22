package attacher

import (
	"io"
	"os"

	"k8s.io/client-go/rest"
)

func FakeAttach(out io.Writer, cfg rest.Config, namespace, name string, in *os.File, containerName, handshake string) error {
	out.Write([]byte("fake attach"))
	return nil
}
