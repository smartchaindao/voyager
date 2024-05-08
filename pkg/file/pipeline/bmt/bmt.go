// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bmt

import (
	"errors"

	"github.com/yanhuangpai/voyager/pkg/bmtpool"
	"github.com/yanhuangpai/voyager/pkg/file/pipeline"
	"github.com/yanhuangpai/voyager/pkg/infinity"
)

var (
	errInvalidData = errors.New("bmt: invalid data")
)

type bmtWriter struct {
	next pipeline.ChainWriter
}

// NewBmtWriter returns a new bmtWriter. Partial writes are not supported.
// Note: branching factor is the BMT branching factor, not the merkle trie branching factor.
func NewBmtWriter(next pipeline.ChainWriter) pipeline.ChainWriter {
	return &bmtWriter{
		next: next,
	}
}

// ChainWrite writes data in chain. It assumes span has voyagern prepended to the data.
// The span can be encrypted or unencrypted.
func (w *bmtWriter) ChainWrite(p *pipeline.PipeWriteArgs) error {
	if len(p.Data) < infinity.SpanSize {
		return errInvalidData
	}
	hasher := bmtpool.Get()
	err := hasher.SetSpanBytes(p.Data[:infinity.SpanSize])
	if err != nil {
		bmtpool.Put(hasher)
		return err
	}
	_, err = hasher.Write(p.Data[infinity.SpanSize:])
	if err != nil {
		bmtpool.Put(hasher)
		return err
	}
	p.Ref = hasher.Sum(nil)
	bmtpool.Put(hasher)

	return w.next.ChainWrite(p)
}

// sum calls the next writer for the cryptographic sum
func (w *bmtWriter) Sum() ([]byte, error) {
	return w.next.Sum()
}
