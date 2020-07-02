package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/util/interrupt"
)

// Workaround wrapper for viper.Unmarshal issue:
// https://github.com/spf13/viper/issues/761
func unmarshalV(config interface{}) error {
	// Marshal struct into format viper can read
	b, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	viper.SetConfigType("yaml")
	if err = viper.ReadConfig(bytes.NewReader(b)); err != nil {
		return err
	}

	if err = viper.Unmarshal(config); err != nil {
		return err
	}

	return nil
}

// Return kube config given its path. If path is "" then use default path location
func configFromPath(path string) (*rest.Config, error) {
	if path == "" {
		path, err := defaultKubeConfigPath()
		if err != nil {
			return nil, err
		}
		return clientcmd.BuildConfigFromFlags("", path)
	} else {
		return clientcmd.BuildConfigFromFlags("", path)
	}
}

// Return default path of kube config file
func defaultKubeConfigPath() (string, error) {
	// Find home directory.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "config"), nil
}

// Wrapper for watchtools.UntilWithSync
func waitUntil(rc rest.Interface, obj runtime.Object, name, namespace, plural string, exitCondition watchtools.ConditionFunc, timeout time.Duration) (runtime.Object, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
	lw := cache.NewListWatchFromClient(rc, plural, namespace, fieldSelector)

	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()

	intr := interrupt.New(nil, cancel)

	var result runtime.Object
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, obj, nil, exitCondition)
		if ev != nil {
			result = ev.Object
		}
		return err
	})
	return result, err
}

// draw divider the width of the terminal
func drawDivider() {
	width := 80

	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		width, _, _ = terminal.GetSize(0)
	}
	fmt.Println(strings.Repeat("-", width))
}
