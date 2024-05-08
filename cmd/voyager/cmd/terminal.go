// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"
)

type passwordReader interface {
	ReadPassword() (password string, err error)
}

type stdInPasswordReader struct{}

func (stdInPasswordReader) ReadPassword() (password string, err error) {
	v, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	return string(v), err
}

func terminalPromptPassword(r passwordReader, title string) (password string, err error) {
	fmt.Println("Please entry the password:")
	password, err = r.ReadPassword()
	if err != nil {
		return "", err
	}
	return password, nil
}

func terminalPromptCreatePassword(r passwordReader) (password string, err error) {
	// cmd.Println("Voyager node is booting up for the first time. Please provide a new password.")
	fmt.Println("Voyager node is booting up for the first time. Please provide a new password.")
	p1, err := terminalPromptPassword(r, "Password")
	if err != nil {
		return "", err
	}

	p2, err := terminalPromptPassword(r, "Confirm password")
	if err != nil {
		return "", err
	}

	if p1 != p2 {
		return "", errors.New("passwords are not the same")
	}

	return p1, nil
}
