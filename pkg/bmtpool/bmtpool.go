// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bmtpool provides easy access to binary
// merkle tree hashers managed in as a resource pool.
package bmtpool

import (
	bmtlegacy "github.com/ethersphere/bmt/legacy"
	"github.com/ethersphere/bmt/pool"
	"github.com/yanhuangpai/voyager/pkg/infinity"
)

var instance pool.Pooler

func init() {
	instance = pool.New(8, infinity.BmtBranches)
}

// Get a bmt Hasher instance.
// Instances are reset before being returned to the caller.
func Get() *bmtlegacy.Hasher {
	return instance.Get()
}

// Put a bmt Hasher back into the pool
func Put(h *bmtlegacy.Hasher) {
	instance.Put(h)
}
