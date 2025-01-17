// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package addressbook

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yanhuangpai/voyager/pkg/ifi"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/storage"
)

const keyPrefix = "addressbook_entry_"

var _ Interface = (*store)(nil)

var ErrNotFound = errors.New("addressbook: not found")

// Interface is the AddressBook interface.
type Interface interface {
	GetPutter
	Remover
	// Overlays returns a list of all overlay addresses saved in addressbook.
	Overlays() ([]infinity.Address, error)
	// Addresses returns a list of all ifi.Address-es saved in addressbook.
	Addresses() ([]ifi.Address, error)
}

type GetPutter interface {
	Getter
	Putter
}

type Getter interface {
	// Get returns pointer to saved ifi.Address for requested overlay address.
	Get(overlay infinity.Address) (addr *ifi.Address, err error)
}

type Putter interface {
	// Put saves relation between peer overlay address and ifi.Address address.
	Put(overlay infinity.Address, addr ifi.Address) (err error)
}

type Remover interface {
	// Remove removes overlay address.
	Remove(overlay infinity.Address) error
}

type store struct {
	store storage.StateStorer
}

// New creates new addressbook for state storer.
func New(storer storage.StateStorer) Interface {
	return &store{
		store: storer,
	}
}

func (s *store) Get(overlay infinity.Address) (*ifi.Address, error) {
	key := keyPrefix + overlay.String()
	v := &ifi.Address{}
	err := s.store.Get(key, &v)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, ErrNotFound
		}

		return nil, err
	}
	return v, nil
}

func (s *store) Put(overlay infinity.Address, addr ifi.Address) (err error) {
	key := keyPrefix + overlay.String()
	return s.store.Put(key, &addr)
}

func (s *store) Remove(overlay infinity.Address) error {
	return s.store.Delete(keyPrefix + overlay.String())
}

func (s *store) Overlays() (overlays []infinity.Address, err error) {
	err = s.store.Iterate(keyPrefix, func(key, _ []byte) (stop bool, err error) {
		k := string(key)
		if !strings.HasPrefix(k, keyPrefix) {
			return true, nil
		}
		split := strings.SplitAfter(k, keyPrefix)
		if len(split) != 2 {
			return true, fmt.Errorf("invalid overlay key: %s", k)
		}
		addr, err := infinity.ParseHexAddress(split[1])
		if err != nil {
			return true, err
		}
		overlays = append(overlays, addr)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return overlays, nil
}

func (s *store) Addresses() (addresses []ifi.Address, err error) {
	err = s.store.Iterate(keyPrefix, func(_, value []byte) (stop bool, err error) {
		entry := &ifi.Address{}
		err = entry.UnmarshalJSON(value)
		if err != nil {
			return true, err
		}

		addresses = append(addresses, *entry)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return addresses, nil
}
