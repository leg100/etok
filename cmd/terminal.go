package cmd

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"
)

// draw divider the width of the terminal
func drawDivider() {
	width := 80

	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		width, _, _ = terminal.GetSize(0)
	}
	fmt.Println(strings.Repeat("-", width))
}
