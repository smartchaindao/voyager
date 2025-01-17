// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mock

import (
	"context"
	"errors"
	"time"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/yanhuangpai/voyager/pkg/ifi"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/p2p"
)

// Service is the mock of a P2P Service
type Service struct {
	addProtocolFunc       func(p2p.ProtocolSpec) error
	connectFunc           func(ctx context.Context, addr ma.Multiaddr) (address *ifi.Address, err error)
	disconnectFunc        func(overlay infinity.Address) error
	peersFunc             func() []p2p.Peer
	blocklistedPeersFunc  func() ([]p2p.Peer, error)
	addressesFunc         func() ([]ma.Multiaddr, error)
	setNotifierFunc       func(p2p.PickyNotifier)
	setWelcomeMessageFunc func(string) error
	getWelcomeMessageFunc func() string
	blocklistFunc         func(infinity.Address, time.Duration) error
	welcomeMessage        string
}

// WithAddProtocolFunc sets the mock implementation of the AddProtocol function
func WithAddProtocolFunc(f func(p2p.ProtocolSpec) error) Option {
	return optionFunc(func(s *Service) {
		s.addProtocolFunc = f
	})
}

// WithSetNotifierFunc sets the mock implementation of the SetNotifier function
func WithSetPickyNotifierFunc(f func(p2p.PickyNotifier)) Option {
	return optionFunc(func(s *Service) {
		s.setNotifierFunc = f
	})
}

// WithConnectFunc sets the mock implementation of the Connect function
func WithConnectFunc(f func(ctx context.Context, addr ma.Multiaddr) (address *ifi.Address, err error)) Option {
	return optionFunc(func(s *Service) {
		s.connectFunc = f
	})
}

// WithDisconnectFunc sets the mock implementation of the Disconnect function
func WithDisconnectFunc(f func(overlay infinity.Address) error) Option {
	return optionFunc(func(s *Service) {
		s.disconnectFunc = f
	})
}

// WithPeersFunc sets the mock implementation of the Peers function
func WithPeersFunc(f func() []p2p.Peer) Option {
	return optionFunc(func(s *Service) {
		s.peersFunc = f
	})
}

// WithBlocklistedPeersFunc sets the mock implementation of the BlocklistedPeers function
func WithBlocklistedPeersFunc(f func() ([]p2p.Peer, error)) Option {
	return optionFunc(func(s *Service) {
		s.blocklistedPeersFunc = f
	})
}

// WithAddressesFunc sets the mock implementation of the Adresses function
func WithAddressesFunc(f func() ([]ma.Multiaddr, error)) Option {
	return optionFunc(func(s *Service) {
		s.addressesFunc = f
	})
}

// WithGetWelcomeMessageFunc sets the mock implementation of the GetWelcomeMessage function
func WithGetWelcomeMessageFunc(f func() string) Option {
	return optionFunc(func(s *Service) {
		s.getWelcomeMessageFunc = f
	})
}

// WithSetWelcomeMessageFunc sets the mock implementation of the SetWelcomeMessage function
func WithSetWelcomeMessageFunc(f func(string) error) Option {
	return optionFunc(func(s *Service) {
		s.setWelcomeMessageFunc = f
	})
}

func WithBlocklistFunc(f func(infinity.Address, time.Duration) error) Option {
	return optionFunc(func(s *Service) {
		s.blocklistFunc = f
	})
}

// New will create a new mock P2P Service with the given options
func New(opts ...Option) *Service {
	s := new(Service)
	for _, o := range opts {
		o.apply(s)
	}
	return s
}

func (s *Service) AddProtocol(spec p2p.ProtocolSpec) error {
	if s.addProtocolFunc == nil {
		return errors.New("function AddProtocol not configured")
	}
	return s.addProtocolFunc(spec)
}

func (s *Service) Connect(ctx context.Context, addr ma.Multiaddr) (address *ifi.Address, err error) {
	if s.connectFunc == nil {
		return nil, errors.New("function Connect not configured")
	}
	return s.connectFunc(ctx, addr)
}

func (s *Service) Disconnect(overlay infinity.Address) error {
	if s.disconnectFunc == nil {
		return errors.New("function Disconnect not configured")
	}
	return s.disconnectFunc(overlay)
}

func (s *Service) Addresses() ([]ma.Multiaddr, error) {
	if s.addressesFunc == nil {
		return nil, errors.New("function Addresses not configured")
	}
	return s.addressesFunc()
}

func (s *Service) Peers() []p2p.Peer {
	if s.peersFunc == nil {
		return nil
	}
	return s.peersFunc()
}

func (s *Service) BlocklistedPeers() ([]p2p.Peer, error) {
	if s.blocklistedPeersFunc == nil {
		return nil, nil
	}

	return s.blocklistedPeersFunc()
}

func (s *Service) SetWelcomeMessage(val string) error {
	if s.setWelcomeMessageFunc != nil {
		return s.setWelcomeMessageFunc(val)
	}
	s.welcomeMessage = val
	return nil
}

func (s *Service) GetWelcomeMessage() string {
	if s.getWelcomeMessageFunc != nil {
		return s.getWelcomeMessageFunc()
	}
	return s.welcomeMessage
}

func (s *Service) Blocklist(overlay infinity.Address, duration time.Duration) error {
	if s.blocklistFunc == nil {
		return errors.New("function blocklist not configured")
	}
	return s.blocklistFunc(overlay, duration)
}

func (s *Service) SetPickyNotifier(f p2p.PickyNotifier) {
	if s.setNotifierFunc == nil {
		return
	}

	s.setNotifierFunc(f)
}

type Option interface {
	apply(*Service)
}
type optionFunc func(*Service)

func (f optionFunc) apply(r *Service) { f(r) }
