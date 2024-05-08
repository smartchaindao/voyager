// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"errors"

	"github.com/yanhuangpai/voyager/pkg/file/pipeline"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/sctx"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"github.com/yanhuangpai/voyager/pkg/tags"
)

var errInvalidData = errors.New("store: invalid data")

type storeWriter struct {
	l    storage.Putter
	mode storage.ModePut
	ctx  context.Context
	next pipeline.ChainWriter
}

// NewStoreWriter returns a storeWriter. It just writes the given data
// to a given storage.Putter.
func NewStoreWriter(ctx context.Context, l storage.Putter, mode storage.ModePut, next pipeline.ChainWriter) pipeline.ChainWriter {
	return &storeWriter{ctx: ctx, l: l, mode: mode, next: next}
}

func (w *storeWriter) ChainWrite(p *pipeline.PipeWriteArgs) error {
	if p.Ref == nil || p.Data == nil {
		return errInvalidData
	}
	tag := sctx.GetTag(w.ctx)
	var c infinity.Chunk
	if tag != nil {
		err := tag.Inc(tags.StateSplit)
		if err != nil {
			return err
		}
		c = infinity.NewChunk(infinity.NewAddress(p.Ref), p.Data).WithTagID(tag.Uid)
	} else {
		c = infinity.NewChunk(infinity.NewAddress(p.Ref), p.Data)
	}
	seen, err := w.l.Put(w.ctx, w.mode, c)
	if err != nil {
		return err
	}
	if tag != nil {
		err := tag.Inc(tags.StateStored)
		if err != nil {
			return err
		}
		if seen[0] {
			err := tag.Inc(tags.StateSeen)
			if err != nil {
				return err
			}
		}
	}
	if w.next == nil {
		return nil
	}

	return w.next.ChainWrite(p)

}

func (w *storeWriter) Sum() ([]byte, error) {
	return w.next.Sum()
}
