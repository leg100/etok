package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"
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

// draw divider the width of the terminal
func drawDivider() {
	width := 80

	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		width, _, _ = terminal.GetSize(0)
	}
	fmt.Println(strings.Repeat("-", width))
}
