// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ifi_test

import (
	"testing"

	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/ifi"

	ma "github.com/multiformats/go-multiaddr"
)

func TestIfiAddress(t *testing.T) {
	node1ma, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/11634/p2p/16Uiu2HAkx8ULY8cTXhdVAcMmLcH9AsTKz6uBQ7DPLKRjMLgBVYkA")
	if err != nil {
		t.Fatal(err)
	}

	privateKey1, err := crypto.GenerateSecp256k1Key()
	if err != nil {
		t.Fatal(err)
	}

	overlay, err := crypto.NewOverlayAddress(privateKey1.PublicKey, 3)
	if err != nil {
		t.Fatal(err)
	}
	signer1 := crypto.NewDefaultSigner(privateKey1)

	ifiAddress, err := ifi.NewAddress(signer1, node1ma, overlay, 3)
	if err != nil {
		t.Fatal(err)
	}

	ifiAddress2, err := ifi.ParseAddress(node1ma.Bytes(), overlay.Bytes(), ifiAddress.Signature, 3)
	if err != nil {
		t.Fatal(err)
	}

	if !ifiAddress.Equal(ifiAddress2) {
		t.Fatalf("got %s expected %s", ifiAddress2, ifiAddress)
	}

	bytes, err := ifiAddress.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	var newifi ifi.Address
	if err := newifi.UnmarshalJSON(bytes); err != nil {
		t.Fatal(err)
	}

	if !newifi.Equal(ifiAddress) {
		t.Fatalf("got %s expected %s", newifi, ifiAddress)
	}
}
