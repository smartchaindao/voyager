// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package discovery exposes the discovery driver interface
// which is implemented by discovery protocols.
package discovery

import (
	"context"

	"github.com/yanhuangpai/voyager/pkg/infinity"
)

type Driver interface {
	BroadcastPeers(ctx context.Context, addressee infinity.Address, peers ...infinity.Address) error
}
