// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mock provides a mock implementation for the
// accounting interface.
package mock

import (
	"context"
	"math/big"
	"sync"

	"github.com/yanhuangpai/voyager/pkg/accounting"
	"github.com/yanhuangpai/voyager/pkg/infinity"
)

// Service is the mock Accounting service.
type Service struct {
	lock                    sync.Mutex
	balances                map[string]*big.Int
	reserveFunc             func(ctx context.Context, peer infinity.Address, price uint64) error
	releaseFunc             func(peer infinity.Address, price uint64)
	creditFunc              func(peer infinity.Address, price uint64) error
	debitFunc               func(peer infinity.Address, price uint64) error
	balanceFunc             func(infinity.Address) (*big.Int, error)
	balancesFunc            func() (map[string]*big.Int, error)
	compensatedBalanceFunc  func(infinity.Address) (*big.Int, error)
	compensatedBalancesFunc func() (map[string]*big.Int, error)

	balanceSurplusFunc func(infinity.Address) (*big.Int, error)
}

// WithReserveFunc sets the mock Reserve function
func WithReserveFunc(f func(ctx context.Context, peer infinity.Address, price uint64) error) Option {
	return optionFunc(func(s *Service) {
		s.reserveFunc = f
	})
}

// WithReleaseFunc sets the mock Release function
func WithReleaseFunc(f func(peer infinity.Address, price uint64)) Option {
	return optionFunc(func(s *Service) {
		s.releaseFunc = f
	})
}

// WithCreditFunc sets the mock Credit function
func WithCreditFunc(f func(peer infinity.Address, price uint64) error) Option {
	return optionFunc(func(s *Service) {
		s.creditFunc = f
	})
}

// WithDebitFunc sets the mock Debit function
func WithDebitFunc(f func(peer infinity.Address, price uint64) error) Option {
	return optionFunc(func(s *Service) {
		s.debitFunc = f
	})
}

// WithBalanceFunc sets the mock Balance function
func WithBalanceFunc(f func(infinity.Address) (*big.Int, error)) Option {
	return optionFunc(func(s *Service) {
		s.balanceFunc = f
	})
}

// WithBalancesFunc sets the mock Balances function
func WithBalancesFunc(f func() (map[string]*big.Int, error)) Option {
	return optionFunc(func(s *Service) {
		s.balancesFunc = f
	})
}

// WithCompensatedBalanceFunc sets the mock Balance function
func WithCompensatedBalanceFunc(f func(infinity.Address) (*big.Int, error)) Option {
	return optionFunc(func(s *Service) {
		s.compensatedBalanceFunc = f
	})
}

// WithCompensatedBalancesFunc sets the mock Balances function
func WithCompensatedBalancesFunc(f func() (map[string]*big.Int, error)) Option {
	return optionFunc(func(s *Service) {
		s.compensatedBalancesFunc = f
	})
}

// WithBalanceSurplusFunc sets the mock SurplusBalance function
func WithBalanceSurplusFunc(f func(infinity.Address) (*big.Int, error)) Option {
	return optionFunc(func(s *Service) {
		s.balanceSurplusFunc = f
	})
}

// NewAccounting creates the mock accounting implementation
func NewAccounting(opts ...Option) accounting.Interface {
	mock := new(Service)
	mock.balances = make(map[string]*big.Int)
	for _, o := range opts {
		o.apply(mock)
	}
	return mock
}

// Reserve is the mock function wrapper that calls the set implementation
func (s *Service) Reserve(ctx context.Context, peer infinity.Address, price uint64) error {
	if s.reserveFunc != nil {
		return s.reserveFunc(ctx, peer, price)
	}
	return nil
}

// Release is the mock function wrapper that calls the set implementation
func (s *Service) Release(peer infinity.Address, price uint64) {
	if s.releaseFunc != nil {
		s.releaseFunc(peer, price)
	}
}

// Credit is the mock function wrapper that calls the set implementation
func (s *Service) Credit(peer infinity.Address, price uint64) error {
	if s.creditFunc != nil {
		return s.creditFunc(peer, price)
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	if bal, ok := s.balances[peer.String()]; ok {
		s.balances[peer.String()] = new(big.Int).Sub(bal, new(big.Int).SetUint64(price))
	} else {
		s.balances[peer.String()] = big.NewInt(-int64(price))
	}
	return nil
}

// Debit is the mock function wrapper that calls the set implementation
func (s *Service) Debit(peer infinity.Address, price uint64) error {
	if s.debitFunc != nil {
		return s.debitFunc(peer, price)
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	if bal, ok := s.balances[peer.String()]; ok {
		s.balances[peer.String()] = new(big.Int).Add(bal, new(big.Int).SetUint64(price))
	} else {
		s.balances[peer.String()] = new(big.Int).SetUint64(price)
	}
	return nil
}

// Balance is the mock function wrapper that calls the set implementation
func (s *Service) Balance(peer infinity.Address) (*big.Int, error) {
	if s.balanceFunc != nil {
		return s.balanceFunc(peer)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	if bal, ok := s.balances[peer.String()]; ok {
		return bal, nil
	} else {
		return big.NewInt(0), nil
	}
}

// Balances is the mock function wrapper that calls the set implementation
func (s *Service) Balances() (map[string]*big.Int, error) {
	if s.balancesFunc != nil {
		return s.balancesFunc()
	}
	return s.balances, nil
}

// CompensatedBalance is the mock function wrapper that calls the set implementation
func (s *Service) CompensatedBalance(peer infinity.Address) (*big.Int, error) {
	if s.compensatedBalanceFunc != nil {
		return s.compensatedBalanceFunc(peer)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.balances[peer.String()], nil
}

// CompensatedBalances is the mock function wrapper that calls the set implementation
func (s *Service) CompensatedBalances() (map[string]*big.Int, error) {
	if s.compensatedBalancesFunc != nil {
		return s.compensatedBalancesFunc()
	}
	return s.balances, nil
}

func (s *Service) SurplusBalance(peer infinity.Address) (*big.Int, error) {
	if s.balanceFunc != nil {
		return s.balanceSurplusFunc(peer)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	return big.NewInt(0), nil
}

// Option is the option passed to the mock accounting service
type Option interface {
	apply(*Service)
}

type optionFunc func(*Service)

func (f optionFunc) apply(r *Service) { f(r) }
