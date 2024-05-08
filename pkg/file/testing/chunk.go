// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"encoding/binary"
	"math/rand"

	"github.com/yanhuangpai/voyager/pkg/infinity"
)

// GenerateTestRandomFileChunk generates one single chunk with arbitrary content and address
func GenerateTestRandomFileChunk(address infinity.Address, spanLength, dataSize int) infinity.Chunk {
	data := make([]byte, dataSize+8)
	binary.LittleEndian.PutUint64(data, uint64(spanLength))
	_, _ = rand.Read(data[8:]) // # skipcq: GSC-G404
	key := make([]byte, infinity.SectionSize)
	if address.IsZero() {
		_, _ = rand.Read(key) // # skipcq: GSC-G404
	} else {
		copy(key, address.Bytes())
	}
	return infinity.NewChunk(infinity.NewAddress(key), data)
}
