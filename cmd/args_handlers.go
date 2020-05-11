package cmd

import (
	"strings"

	"github.com/leg100/stok/util/slice"
)

func DoubleDashArgsHandler(args []string) []string {
	if i := slice.StringIndex(args, "--"); i > -1 {
		return args[i+1:]
	} else {
		return []string{}
	}
}

func ShellWrapDoubleDashArgsHandler(args []string) []string {
	if i := slice.StringIndex(args, "--"); i > -1 {
		cflag := []string{"-c"}
		joined := strings.Join(args[i+1:], " ")
		return append(cflag, "\""+joined+"\"")
	} else {
		return []string{}
	}
}
