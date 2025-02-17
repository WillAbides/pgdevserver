package pgdevserver

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// appendFlagArg appends flag and value to args if value is not empty
func appendFlagArg(args []string, flag, value string) []string {
	if value == "" {
		return args
	}
	return append(args, flag, value)
}

func availableTcpPort(preferred string) (_ string, errOut error) {
	addr := fmt.Sprintf(":%s", preferred)
	if preferred == "" {
		addr = ":0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if preferred != "" {
			return availableTcpPort("")
		}
		return "", err
	}
	err = ln.Close()
	if err != nil {
		return "", err
	}
	str := ln.Addr().String()
	return str[strings.LastIndex(str, ":")+1:], nil
}

// execRun is cmd.Run except the ExitError is populated with both stdout and stderr
func execRun(cmd *exec.Cmd) error {
	var exitErr *exec.ExitError
	b, err := cmd.CombinedOutput()
	if errors.As(err, &exitErr) {
		exitErr.Stderr = b
		err = exitErr
	}
	return err
}
