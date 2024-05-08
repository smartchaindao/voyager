// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package addressbook_test

import (
	"testing"

	"github.com/yanhuangpai/voyager/pkg/addressbook"
	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/ifi"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/statestore/mock"

	ma "github.com/multiformats/go-multiaddr"
)

type bookFunc func(t *testing.T) (book addressbook.Interface)

func TestInMem(t *testing.T) {
	run(t, func(t *testing.T) addressbook.Interface {
		store := mock.NewStateStore()
		book := addressbook.New(store)
		return book
	})
}

func run(t *testing.T, f bookFunc) {
	store := f(t)
	addr1 := infinity.NewAddress([]byte{0, 1, 2, 3})
	addr2 := infinity.NewAddress([]byte{0, 1, 2, 4})
	multiaddr, err := ma.NewMultiaddr("/ip4/1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}

	pk, err := crypto.GenerateSecp256k1Key()
	if err != nil {
		t.Fatal(err)
	}

	ifiAddr, err := ifi.NewAddress(crypto.NewDefaultSigner(pk), multiaddr, addr1, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = store.Put(addr1, *ifiAddr)
	if err != nil {
		t.Fatal(err)
	}

	v, err := store.Get(addr1)
	if err != nil {
		t.Fatal(err)
	}

	if !ifiAddr.Equal(v) {
		t.Fatalf("expectted: %s, want %s", v, multiaddr)
	}

	notFound, err := store.Get(addr2)
	if err != addressbook.ErrNotFound {
		t.Fatal(err)
	}

	if notFound != nil {
		t.Fatalf("expected nil got %s", v)
	}

	overlays, err := store.Overlays()
	if err != nil {
		t.Fatal(err)
	}

	if len(overlays) != 1 {
		t.Fatalf("expected overlay len %v, got %v", 1, len(overlays))
	}

	addresses, err := store.Addresses()
	if err != nil {
		t.Fatal(err)
	}

	if len(addresses) != 1 {
		t.Fatalf("expected addresses len %v, got %v", 1, len(addresses))
	}
}
