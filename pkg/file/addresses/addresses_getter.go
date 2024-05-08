// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package addresses

import (
	"context"

	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/storage"
)

type addressesGetterStore struct {
	getter storage.Getter
	fn     infinity.AddressIterFunc
}

// NewGetter creates a new proxy storage.Getter which calls provided function
// for each chunk address processed.
func NewGetter(getter storage.Getter, fn infinity.AddressIterFunc) storage.Getter {
	return &addressesGetterStore{getter, fn}
}

func (s *addressesGetterStore) Get(ctx context.Context, mode storage.ModeGet, addr infinity.Address) (infinity.Chunk, error) {
	ch, err := s.getter.Get(ctx, mode, addr)
	if err != nil {
		return nil, err
	}

	return ch, s.fn(ch.Address())
}
