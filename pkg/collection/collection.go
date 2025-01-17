// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package collection provides high-level abstractions for collections of files
package collection

import (
	"github.com/yanhuangpai/voyager/pkg/infinity"
)

// Collection provides a specific ordering of a collection of binary data vectors
// stored in voyager.
type Collection interface {
	Addresses() []infinity.Address
}

// Entry encapsulates data defining a single file entry.
// It may contain any number of data blobs providing context to the
// given data vector concealed by Reference.
type Entry interface {
	Reference() infinity.Address
	Metadata() infinity.Address
}
