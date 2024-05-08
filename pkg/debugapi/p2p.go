// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debugapi

import (
	"encoding/hex"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/multiformats/go-multiaddr"
	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp"
)

type addressesResponse struct {
	Overlay      infinity.Address      `json:"overlay"`
	Underlay     []multiaddr.Multiaddr `json:"underlay"`
	Ethereum     common.Address        `json:"ifiaddress"`
	PublicKey    string                `json:"public_key"`
	PSSPublicKey string                `json:"pss_public_key"`
}

func (s *Service) addressesHandler(w http.ResponseWriter, r *http.Request) {
	// initialize variable to json encode as [] instead null if p2p is nil
	underlay := make([]multiaddr.Multiaddr, 0)
	// addresses endpoint is exposed before p2p service is configured
	// to provide information about other addresses.
	if s.p2p != nil {
		u, err := s.p2p.Addresses()
		if err != nil {
			s.logger.Debugf("debug api: p2p addresses: %v", err)
			jsonhttp.InternalServerError(w, err)
			return
		}
		underlay = u
	}
	jsonhttp.OK(w, addressesResponse{
		Overlay:      s.overlay,
		Underlay:     underlay,
		Ethereum:     s.ethereumAddress,
		PublicKey:    hex.EncodeToString(crypto.EncodeSecp256k1PublicKey(&s.publicKey)),
		PSSPublicKey: hex.EncodeToString(crypto.EncodeSecp256k1PublicKey(&s.pssPublicKey)),
	})
}
