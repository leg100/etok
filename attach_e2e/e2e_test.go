package attach_e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/pkg/podhandler"
	"github.com/leg100/stok/pkg/signals"
	"github.com/leg100/stok/util"
	"github.com/moby/term"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Attach to pod End-to-end tests
func TestAttach(t *testing.T) {
	log.Infof("is stdout a terminal? %v\n", term.IsTerminal(os.Stdout.Fd()))
	log.Infof("is stdin a terminal? %v\n", term.IsTerminal(os.Stdin.Fd()))
	// Create context, and cancel if interrupt is received
	ctx, cancel := context.WithCancel(context.Background())
	signals.CatchCtrlC(cancel)

	//log.Infof("current path: %s\n", os.Getwd

	// Build bin first
	//cmd := exec.Command("go", "build", "-o", "./attacher/attacher", "./attacher")
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	//require.NoError(t, cmd.Run())

	// Fetch default options
	opts, err := app.NewOpts()
	require.NoError(t, err)

	// Create k8s clients
	require.NoError(t, opts.CreateClients(""))

	// Generate unique pod name and namespace
	podname := fmt.Sprintf("attach-%s", util.GenerateRandomString(5))
	namespace := fmt.Sprintf("attach-%s", util.GenerateRandomString(5))

	// Create ns
	_, err = opts.KubeClient().CoreV1().Namespaces().Create(ctx, testNamespace(namespace), metav1.CreateOptions{})
	require.NoError(t, err)
	log.Infof("created namespace %s\n", namespace)
	defer func() {
		assert.NoError(t, opts.KubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{}))
		log.Infof("deleted namespace %s\n", namespace)
	}()

	// Create pod
	pod, err := opts.KubeClient().CoreV1().Pods(namespace).Create(ctx, testPod(namespace, podname), metav1.CreateOptions{})
	require.NoError(t, err)
	log.Infof("created pod %s/%s\n", namespace, podname)
	defer func() {
		assert.NoError(t, opts.KubeClient().CoreV1().Pods(namespace).Delete(context.Background(), podname, metav1.DeleteOptions{}))
		log.Infof("deleted pod %s/%s\n", namespace, podname)
	}()

	ptm, pts, err := pty.Open()
	require.NoError(t, err)
	defer ptm.Close()
	pty.i

	log.Infof("is tty a terminal? %v\n", term.IsTerminal(c.Tty().Fd()))
	log.Infof("is stdin a terminal? %v\n", term.IsTerminal(os.Stdin.Fd()))

	errch := make(chan error)
	ready := make(chan struct{})

	// Watch for pod events
	(&podMonitor{
		namespace: namespace,
		name:      podname,
		client:    opts.KubeClient(),
	}).monitor(ctx, errch, ready)

	writer := new(bytes.Buffer)

	log.Info("waiting for pod to be ready...")
	// Wait for pod to be ready
	select {
	case <-ready:
		// Attach to pod
		log.Info("attaching to pod...")
		go func() {
			err = (&podhandler.PodHandler{}).Attach(opts.KubeConfig(), pod, c.Tty(), c.Tty(), os.Stderr)
			if err != nil {
				log.Info(err.Error())
			}
			require.NoError(t, err)
			log.Info("detached from pod")
		}()
	//case <-time.After(10 * time.Second):
	//	require.NoError(t, fmt.Errorf("timed out waiting for pod to be ready"))
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	case err := <-errch:
		require.NoError(t, err)
	}
	//cmd = exec.Command("./attacher/attacher", "-name", podname, "-namespace", namespace)
	//cmd.Stdin = c.Tty()
	//cmd.Stdout = c.Tty()
	//cmd.Stderr = c.Tty()

	//require.NoError(t, cmd.Start())
	//go func() {
	//	require.NoError(t, cmd.Wait())
	//}()

	//log.Infof("PID: %d\n", cmd.Process.Pid)

	time.Sleep(time.Second*2)
	stdinR, stdinW := io.Pipe()
	go io.Copy(c.Tty(), stdinR)
	stdinW.Write([]byte("\n"))
	stdinW.Write([]byte("Louis"))

	writer.WriteString("\n")
	writer.WriteString("Louis\n")
	fmt.Fprintf(c.Tty(), "\n")
	fmt.Fprintf(c.Tty(), "Louis\n")
	//c.ExpectString("Enter name: ")
	//c.SendLine("Louis")
	c.SendLine("")
	c.SendLine("")
	c.SendLine("")
	c.ExpectEOF()
	//c.ExpectString("Hello Louis")

	// Just for presentation
	fmt.Println()
}

func testPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:                     "runner",
					Image:                    "alpine",
					ImagePullPolicy:          corev1.PullIfNotPresent,
					Command:                  []string{"sh", "-c", "read -p 'Enter name: ' name; echo Hello $name"},
					Stdin:                    true,
					TTY:                      true,
					TerminationMessagePolicy: "FallbackToLogsOnError",
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func testNamespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

//	tests := []struct {
//		name            string
//		args            []string
//		path            string
//		wantExitCode    int
//		wantStdoutRegex *regexp.Regexp
//		stdin           []byte
//		queueAdditional int
//	}{
//		{
//			name:            "pty",
//			args:            []string{"plan", "-input=true", "-no-color", "--", "--context", *kubectx},
//			wantExitCode:    0,
//			wantStdoutRegex: regexp.MustCompile(`(?s)var\.suffix.*Enter a value:.*Refreshing Terraform state in-memory prior to plan`),
//			stdin:           []byte("foo\n"),
//		},
//	}
//
//	// Invoke stok with each test case
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			cmd := exec.Command(buildPath, tt.args...)
//			cmd.Dir = workspacePath
//
//			outbuf := new(bytes.Buffer)
//			out := io.MultiWriter(outbuf, os.Stdout)
//
//			errbuf := new(bytes.Buffer)
//			stderr := io.MultiWriter(errbuf, os.Stderr)
//
//			if tt.pty {
//				terminal, err := pty.Start(cmd)
//				if err != nil {
//					t.Fatal(err)
//				}
//				defer terminal.Close()
//
//				// https://github.com/creack/pty/issues/82#issuecomment-502785533
//				echoOff(terminal)
//				stdinR, stdinW := io.Pipe()
//				go io.Copy(terminal, stdinR)
//				stdinW.Write(tt.stdin)
//
//				// ... and the pty to stdout.
//				_, _ = io.Copy(out, terminal)
//			} else {
//				// without pty, so just use a buffer, and skip stdin
//				cmd.Stdout = out
//				cmd.Stderr = stderr
//
//				if err = cmd.Start(); err != nil {
//					t.Fatal(err)
//				}
//			}
//
//			exitCodeTest(t, cmd.Wait(), tt.wantExitCode)
//
//			// Without a pty we expect a warning log msg telling us as much.
//			// (We can use stderr without pty but not with pty)
//			if !tt.pty {
//				got := errbuf.String()
//				for _, want := range tt.wantWarnings {
//					if !regexp.MustCompile(want).MatchString(got) {
//						t.Errorf("want '%s', got '%s'\n", want, got)
//					}
//				}
//			}
//
//			got := outbuf.String()
//			if !tt.wantStdoutRegex.MatchString(got) {
//				t.Errorf("expected stdout to match '%s' but got '%s'\n", tt.wantStdoutRegex, got)
//			}
//		})
//	}
//}
//
//func echoOff(f *os.File) {
//	fd := int(f.Fd())
//	//      const ioctlReadTermios = unix.TIOCGETA // OSX.
//	const ioctlReadTermios = unix.TCGETS // Linux
//	//      const ioctlWriterTermios =  unix.TIOCSETA // OSX.
//	const ioctlWriteTermios = unix.TCSETS // Linux
//
//	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
//	if err != nil {
//		panic(err)
//	}
//
//	newState := *termios
//	newState.Lflag &^= unix.ECHO
//	newState.Lflag |= unix.ICANON | unix.ISIG
//	newState.Iflag |= unix.ICRNL
//	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, &newState); err != nil {
//		panic(err)
//	}
//}
//
//func exitCodeTest(t *testing.T, err error, wantExitCode int) {
//	if exiterr, ok := err.(*exec.ExitError); ok {
//		if wantExitCode != exiterr.ExitCode() {
//			t.Fatalf("expected exit code %d, got %d\n", wantExitCode, exiterr.ExitCode())
//		}
//	} else if err != nil {
//		t.Fatal(err)
//	} else {
//		// got exit code 0 and no error
//		if wantExitCode != 0 {
//			t.Fatalf("expected exit code %d, got 0\n", wantExitCode)
//		}
//	}
//}
