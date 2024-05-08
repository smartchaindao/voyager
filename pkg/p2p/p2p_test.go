// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package p2p_test

import (
	"testing"

	"github.com/yanhuangpai/voyager/pkg/p2p"
)

func TestNewInfinityStreamName(t *testing.T) {
	want := "/infinity/hive/1.2.0/peers"
	got := p2p.NewInfinityStreamName("hive", "1.2.0", "peers")

	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
