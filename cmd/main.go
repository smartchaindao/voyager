//go:generate resource/goversioninfo -icon=resource/logo.ico -manifest=resource/goversioninfo.exe.manifest
// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/yanhuangpai/voyager/cmd/voyager/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
		os.Exit(1)
	}
}
