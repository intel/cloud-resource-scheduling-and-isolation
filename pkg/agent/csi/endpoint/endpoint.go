/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

package endpoint

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func ValidateAndParse(ed string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ed), "tcp://") || strings.HasPrefix(strings.ToLower(ed), "unix://") {
		s := strings.SplitN(ed, "://", 2)
		if len(s) >= 2 {
			if s[1] != "" {
				return s[0], s[1], nil
			}
		}
		return "", "", fmt.Errorf("invalid endpoint: %v", ed)
	}
	return "unix", ed, nil
}

func Listen(ep string) (net.Listener, func(), error) {
	proto, addr, err := ValidateAndParse(ep)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {}
	if proto == "unix" {
		addr = "/" + addr
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%s: %q", addr, err)
		}
		cleanup = func() {
			os.Remove(addr)
		}
	}

	l, err := net.Listen(proto, addr)
	return l, cleanup, err
}
