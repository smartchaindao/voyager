// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"github.com/yanhuangpai/voyager/pkg/traversal"
)

// pinFile is used to pin an already uploaded content.
func (s *server) pinFile(w http.ResponseWriter, r *http.Request) {
	addr, err := infinity.ParseHexAddress(mux.Vars(r)["address"])
	if err != nil {
		s.logger.Debugf("pin files: parse address: %v", err)
		s.logger.Error("pin files: parse address")
		jsonhttp.BadRequest(w, "bad address")
		return
	}

	has, err := s.storer.Has(r.Context(), addr)
	if err != nil {
		s.logger.Debugf("pin files: localstore has: %v", err)
		s.logger.Error("pin files: store")
		jsonhttp.InternalServerError(w, err)
		return
	}

	if !has {
		_, err := s.storer.Get(r.Context(), storage.ModeGetRequest, addr)
		if err != nil {
			s.logger.Debugf("pin chunk: netstore get: %v", err)
			s.logger.Error("pin chunk: netstore")

			jsonhttp.NotFound(w, nil)
			return
		}
	}

	ctx := r.Context()

	chunkAddressFn := s.pinChunkAddressFn(ctx, addr)

	err = s.traversal.TraverseFileAddresses(ctx, addr, chunkAddressFn)
	if err != nil {
		s.logger.Debugf("pin files: traverse chunks: %v, addr %s", err, addr)

		if errors.Is(err, traversal.ErrInvalidType) {
			s.logger.Error("pin files: invalid type")
			jsonhttp.BadRequest(w, "invalid type")
			return
		}

		s.logger.Error("pin files: cannot pin")
		jsonhttp.InternalServerError(w, "cannot pin")
		return
	}

	jsonhttp.OK(w, nil)
}

// unpinFile removes pinning from content.
func (s *server) unpinFile(w http.ResponseWriter, r *http.Request) {
	addr, err := infinity.ParseHexAddress(mux.Vars(r)["address"])
	if err != nil {
		s.logger.Debugf("pin files: parse address: %v", err)
		s.logger.Error("pin files: parse address")
		jsonhttp.BadRequest(w, "bad address")
		return
	}

	has, err := s.storer.Has(r.Context(), addr)
	if err != nil {
		s.logger.Debugf("pin files: localstore has: %v", err)
		s.logger.Error("pin files: store")
		jsonhttp.InternalServerError(w, err)
		return
	}

	if !has {
		jsonhttp.NotFound(w, nil)
		return
	}

	ctx := r.Context()

	chunkAddressFn := s.unpinChunkAddressFn(ctx, addr)

	err = s.traversal.TraverseFileAddresses(ctx, addr, chunkAddressFn)
	if err != nil {
		s.logger.Debugf("pin files: traverse chunks: %v, addr %s", err, addr)

		if errors.Is(err, traversal.ErrInvalidType) {
			s.logger.Error("pin files: invalid type")
			jsonhttp.BadRequest(w, "invalid type")
			return
		}

		s.logger.Error("pin files: cannot unpin")
		jsonhttp.InternalServerError(w, "cannot unpin")
		return
	}

	jsonhttp.OK(w, nil)
}
