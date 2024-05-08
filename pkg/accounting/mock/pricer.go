// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mock

import (
	"github.com/yanhuangpai/voyager/pkg/infinity"
)

type MockPricer struct {
	peerPrice uint64
	price     uint64
}

func NewPricer(price, peerPrice uint64) *MockPricer {
	return &MockPricer{
		peerPrice: peerPrice,
		price:     price,
	}
}

func (pricer *MockPricer) PeerPrice(peer, chunk infinity.Address) uint64 {
	return pricer.peerPrice
}

func (pricer *MockPricer) Price(chunk infinity.Address) uint64 {
	return pricer.price
}
