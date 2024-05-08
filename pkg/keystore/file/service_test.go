// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package file_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/yanhuangpai/voyager/pkg/keystore/file"
	"github.com/yanhuangpai/voyager/pkg/keystore/test"
)

func TestService(t *testing.T) {
	dir, err := ioutil.TempDir("", "ifi-keystore-file-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	test.Service(t, file.New(dir))
}
