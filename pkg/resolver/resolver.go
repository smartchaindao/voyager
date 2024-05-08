// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package resolver

import (
	"io"

	"github.com/yanhuangpai/voyager/pkg/infinity"
)

// Address is the Smart Chain ifi address.
type Address = infinity.Address

// Interface can resolve an URL into an associated Ethereum address.
type Interface interface {
	Resolve(url string) (Address, error)
	io.Closer
}
