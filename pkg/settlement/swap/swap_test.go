// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swap_test

import (
	"context"
	"errors"
	"io/ioutil"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/logging"
	mockp2p "github.com/yanhuangpai/voyager/pkg/p2p/mock"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap/chequebook"
	mockchequebook "github.com/yanhuangpai/voyager/pkg/settlement/swap/chequebook/mock"
	mockchequestore "github.com/yanhuangpai/voyager/pkg/settlement/swap/chequestore/mock"
	mockstore "github.com/yanhuangpai/voyager/pkg/statestore/mock"
)

type swapProtocolMock struct {
	emitCheque func(ctx context.Context, peer infinity.Address, cheque *chequebook.SignedCheque) error
}

func (m *swapProtocolMock) EmitCheque(ctx context.Context, peer infinity.Address, cheque *chequebook.SignedCheque) error {
	if m.emitCheque != nil {
		return m.emitCheque(ctx, peer, cheque)
	}
	return nil
}

type testObserver struct {
	called bool
	peer   infinity.Address
	amount *big.Int
}

func (t *testObserver) NotifyPayment(peer infinity.Address, amount *big.Int) error {
	t.called = true
	t.peer = peer
	t.amount = amount
	return nil
}

type addressbookMock struct {
	beneficiary     func(peer infinity.Address) (beneficiary common.Address, known bool, err error)
	chequebook      func(peer infinity.Address) (chequebookAddress common.Address, known bool, err error)
	beneficiaryPeer func(beneficiary common.Address) (peer infinity.Address, known bool, err error)
	chequebookPeer  func(chequebook common.Address) (peer infinity.Address, known bool, err error)
	putBeneficiary  func(peer infinity.Address, beneficiary common.Address) error
	putChequebook   func(peer infinity.Address, chequebook common.Address) error
}

func (m *addressbookMock) Beneficiary(peer infinity.Address) (beneficiary common.Address, known bool, err error) {
	return m.beneficiary(peer)
}
func (m *addressbookMock) Chequebook(peer infinity.Address) (chequebookAddress common.Address, known bool, err error) {
	return m.chequebook(peer)
}
func (m *addressbookMock) BeneficiaryPeer(beneficiary common.Address) (peer infinity.Address, known bool, err error) {
	return m.beneficiaryPeer(beneficiary)
}
func (m *addressbookMock) ChequebookPeer(chequebook common.Address) (peer infinity.Address, known bool, err error) {
	return m.chequebookPeer(chequebook)
}
func (m *addressbookMock) PutBeneficiary(peer infinity.Address, beneficiary common.Address) error {
	return m.putBeneficiary(peer, beneficiary)
}
func (m *addressbookMock) PutChequebook(peer infinity.Address, chequebook common.Address) error {
	return m.putChequebook(peer, chequebook)
}

type cashoutMock struct {
	cashCheque    func(ctx context.Context, chequebook common.Address, recipient common.Address) (common.Hash, error)
	cashoutStatus func(ctx context.Context, chequebookAddress common.Address) (*chequebook.CashoutStatus, error)
}

func (m *cashoutMock) CashCheque(ctx context.Context, chequebook, recipient common.Address) (common.Hash, error) {
	return m.cashCheque(ctx, chequebook, recipient)
}
func (m *cashoutMock) CashoutStatus(ctx context.Context, chequebookAddress common.Address) (*chequebook.CashoutStatus, error) {
	return m.cashoutStatus(ctx, chequebookAddress)
}

func TestReceiveCheque(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()
	chequebookService := mockchequebook.NewChequebook()
	amount := big.NewInt(50)
	chequebookAddress := common.HexToAddress("0xcd")

	peer := infinity.MustParseHexAddress("abcd")
	cheque := &chequebook.SignedCheque{
		Cheque: chequebook.Cheque{
			Beneficiary:      common.HexToAddress("0xab"),
			CumulativePayout: big.NewInt(10),
			Chequebook:       chequebookAddress,
		},
		Signature: []byte{},
	}

	chequeStore := mockchequestore.NewChequeStore(
		mockchequestore.WithRetrieveChequeFunc(func(ctx context.Context, c *chequebook.SignedCheque) (*big.Int, error) {
			if !cheque.Equal(c) {
				t.Fatalf("passed wrong cheque to store. wanted %v, got %v", cheque, c)
			}
			return amount, nil
		}),
	)
	networkID := uint64(1)
	addressbook := &addressbookMock{
		chequebook: func(p infinity.Address) (common.Address, bool, error) {
			if !peer.Equal(p) {
				t.Fatal("querying chequebook for wrong peer")
			}
			return chequebookAddress, true, nil
		},
		putChequebook: func(p infinity.Address, chequebook common.Address) (err error) {
			if !peer.Equal(p) {
				t.Fatal("storing chequebook for wrong peer")
			}
			if chequebook != chequebookAddress {
				t.Fatal("storing wrong chequebook")
			}
			return nil
		},
	}

	swap := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		chequebookService,
		chequeStore,
		addressbook,
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	observer := &testObserver{}
	swap.SetNotifyPaymentFunc(observer.NotifyPayment)

	err := swap.ReceiveCheque(context.Background(), peer, cheque)
	if err != nil {
		t.Fatal(err)
	}

	if !observer.called {
		t.Fatal("expected observer to be called")
	}

	if observer.amount.Cmp(amount) != 0 {
		t.Fatalf("observer called with wrong amount. got %d, want %d", observer.amount, amount)
	}

	if !observer.peer.Equal(peer) {
		t.Fatalf("observer called with wrong peer. got %v, want %v", observer.peer, peer)
	}
}

func TestReceiveChequeReject(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()
	chequebookService := mockchequebook.NewChequebook()
	chequebookAddress := common.HexToAddress("0xcd")

	peer := infinity.MustParseHexAddress("abcd")
	cheque := &chequebook.SignedCheque{
		Cheque: chequebook.Cheque{
			Beneficiary:      common.HexToAddress("0xab"),
			CumulativePayout: big.NewInt(10),
			Chequebook:       chequebookAddress,
		},
		Signature: []byte{},
	}

	var errReject = errors.New("reject")

	chequeStore := mockchequestore.NewChequeStore(
		mockchequestore.WithRetrieveChequeFunc(func(ctx context.Context, c *chequebook.SignedCheque) (*big.Int, error) {
			return nil, errReject
		}),
	)
	networkID := uint64(1)
	addressbook := &addressbookMock{
		chequebook: func(p infinity.Address) (common.Address, bool, error) {
			return chequebookAddress, true, nil
		},
	}

	swap := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		chequebookService,
		chequeStore,
		addressbook,
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	observer := &testObserver{}
	swap.SetNotifyPaymentFunc(observer.NotifyPayment)

	err := swap.ReceiveCheque(context.Background(), peer, cheque)
	if err == nil {
		t.Fatal("accepted invalid cheque")
	}
	if !errors.Is(err, errReject) {
		t.Fatalf("wrong error. wanted %v, got %v", errReject, err)
	}

	if observer.called {
		t.Fatal("observer was be called for rejected payment")
	}
}

func TestReceiveChequeWrongChequebook(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()
	chequebookService := mockchequebook.NewChequebook()
	chequebookAddress := common.HexToAddress("0xcd")

	peer := infinity.MustParseHexAddress("abcd")
	cheque := &chequebook.SignedCheque{
		Cheque: chequebook.Cheque{
			Beneficiary:      common.HexToAddress("0xab"),
			CumulativePayout: big.NewInt(10),
			Chequebook:       chequebookAddress,
		},
		Signature: []byte{},
	}

	chequeStore := mockchequestore.NewChequeStore()
	networkID := uint64(1)
	addressbook := &addressbookMock{
		chequebook: func(p infinity.Address) (common.Address, bool, error) {
			return common.HexToAddress("0xcfff"), true, nil
		},
	}

	swapService := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		chequebookService,
		chequeStore,
		addressbook,
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	observer := &testObserver{}
	swapService.SetNotifyPaymentFunc(observer.NotifyPayment)

	err := swapService.ReceiveCheque(context.Background(), peer, cheque)
	if err == nil {
		t.Fatal("accepted invalid cheque")
	}
	if !errors.Is(err, swap.ErrWrongChequebook) {
		t.Fatalf("wrong error. wanted %v, got %v", swap.ErrWrongChequebook, err)
	}

	if observer.called {
		t.Fatal("observer was be called for rejected payment")
	}
}

func TestPay(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	amount := big.NewInt(50)
	beneficiary := common.HexToAddress("0xcd")
	var cheque chequebook.SignedCheque

	peer := infinity.MustParseHexAddress("abcd")
	var chequebookCalled bool
	chequebookService := mockchequebook.NewChequebook(
		mockchequebook.WithChequebookIssueFunc(func(ctx context.Context, b common.Address, a *big.Int, sendChequeFunc chequebook.SendChequeFunc) (*big.Int, error) {
			if b != beneficiary {
				t.Fatalf("issuing cheque for wrong beneficiary. wanted %v, got %v", beneficiary, b)
			}
			if a.Cmp(amount) != 0 {
				t.Fatalf("issuing cheque with wrong amount. wanted %d, got %d", amount, a)
			}
			chequebookCalled = true
			return big.NewInt(0), sendChequeFunc(&cheque)
		}),
	)

	networkID := uint64(1)
	addressbook := &addressbookMock{
		beneficiary: func(p infinity.Address) (common.Address, bool, error) {
			if !peer.Equal(p) {
				t.Fatal("querying beneficiary for wrong peer")
			}
			return beneficiary, true, nil
		},
	}

	var emitCalled bool
	swap := swap.New(
		&swapProtocolMock{
			emitCheque: func(ctx context.Context, p infinity.Address, c *chequebook.SignedCheque) error {
				if !peer.Equal(p) {
					t.Fatal("sending to wrong peer")
				}
				if !cheque.Equal(c) {
					t.Fatal("sending wrong cheque")
				}
				emitCalled = true
				return nil
			},
		},
		logger,
		store,
		chequebookService,
		mockchequestore.NewChequeStore(),
		addressbook,
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	err := swap.Pay(context.Background(), peer, amount)
	if err != nil {
		t.Fatal(err)
	}

	if !chequebookCalled {
		t.Fatal("chequebook was not called")
	}

	if !emitCalled {
		t.Fatal("swap protocol was not called")
	}
}

func TestPayIssueError(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	amount := big.NewInt(50)
	beneficiary := common.HexToAddress("0xcd")

	peer := infinity.MustParseHexAddress("abcd")
	errReject := errors.New("reject")
	chequebookService := mockchequebook.NewChequebook(
		mockchequebook.WithChequebookIssueFunc(func(ctx context.Context, b common.Address, a *big.Int, sendChequeFunc chequebook.SendChequeFunc) (*big.Int, error) {
			return big.NewInt(0), errReject
		}),
	)

	networkID := uint64(1)
	addressbook := &addressbookMock{
		beneficiary: func(p infinity.Address) (common.Address, bool, error) {
			if !peer.Equal(p) {
				t.Fatal("querying beneficiary for wrong peer")
			}
			return beneficiary, true, nil
		},
	}

	swap := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		chequebookService,
		mockchequestore.NewChequeStore(),
		addressbook,
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	err := swap.Pay(context.Background(), peer, amount)
	if !errors.Is(err, errReject) {
		t.Fatalf("wrong error. wanted %v, got %v", errReject, err)
	}
}

func TestPayUnknownBeneficiary(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	amount := big.NewInt(50)
	peer := infinity.MustParseHexAddress("abcd")
	networkID := uint64(1)
	addressbook := &addressbookMock{
		beneficiary: func(p infinity.Address) (common.Address, bool, error) {
			if !peer.Equal(p) {
				t.Fatal("querying beneficiary for wrong peer")
			}
			return common.Address{}, false, nil
		},
	}

	var disconnectCalled bool
	swapService := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		mockchequebook.NewChequebook(),
		mockchequestore.NewChequeStore(),
		addressbook,
		networkID,
		&cashoutMock{},
		mockp2p.New(
			mockp2p.WithDisconnectFunc(func(disconnectPeer infinity.Address) error {
				if !peer.Equal(disconnectPeer) {
					t.Fatalf("disconnecting wrong peer. wanted %v, got %v", peer, disconnectPeer)
				}
				disconnectCalled = true
				return nil
			}),
		),
	)

	err := swapService.Pay(context.Background(), peer, amount)
	if !errors.Is(err, swap.ErrUnknownBeneficary) {
		t.Fatalf("wrong error. wanted %v, got %v", swap.ErrUnknownBeneficary, err)
	}

	if !disconnectCalled {
		t.Fatal("disconnect was not called")
	}
}

func TestHandshake(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	beneficiary := common.HexToAddress("0xcd")
	networkID := uint64(1)
	peer := crypto.NewOverlayFromEthereumAddress(beneficiary[:], networkID)

	var putCalled bool
	swapService := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		mockchequebook.NewChequebook(),
		mockchequestore.NewChequeStore(),
		&addressbookMock{
			beneficiary: func(p infinity.Address) (common.Address, bool, error) {
				return beneficiary, true, nil
			},
			putBeneficiary: func(p infinity.Address, b common.Address) error {
				putCalled = true
				return nil
			},
		},
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	err := swapService.Handshake(peer, beneficiary)
	if err != nil {
		t.Fatal(err)
	}

	if putCalled {
		t.Fatal("beneficiary was saved again")
	}
}

func TestHandshakeNewPeer(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	beneficiary := common.HexToAddress("0xcd")
	networkID := uint64(1)
	peer := crypto.NewOverlayFromEthereumAddress(beneficiary[:], networkID)

	var putCalled bool
	swapService := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		mockchequebook.NewChequebook(),
		mockchequestore.NewChequeStore(),
		&addressbookMock{
			beneficiary: func(p infinity.Address) (common.Address, bool, error) {
				return beneficiary, false, nil
			},
			putBeneficiary: func(p infinity.Address, b common.Address) error {
				putCalled = true
				return nil
			},
		},
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	err := swapService.Handshake(peer, beneficiary)
	if err != nil {
		t.Fatal(err)
	}

	if !putCalled {
		t.Fatal("beneficiary was not saved")
	}
}

func TestHandshakeWrongBeneficiary(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	beneficiary := common.HexToAddress("0xcd")
	peer := infinity.MustParseHexAddress("abcd")
	networkID := uint64(1)

	swapService := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		mockchequebook.NewChequebook(),
		mockchequestore.NewChequeStore(),
		&addressbookMock{},
		networkID,
		&cashoutMock{},
		mockp2p.New(),
	)

	err := swapService.Handshake(peer, beneficiary)
	if !errors.Is(err, swap.ErrWrongBeneficiary) {
		t.Fatalf("wrong error. wanted %v, got %v", swap.ErrWrongBeneficiary, err)
	}
}

func TestCashout(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	theirChequebookAddress := common.HexToAddress("ffff")
	ourChequebookAddress := common.HexToAddress("fffa")
	peer := infinity.MustParseHexAddress("abcd")
	txHash := common.HexToHash("eeee")
	addressbook := &addressbookMock{
		chequebook: func(p infinity.Address) (common.Address, bool, error) {
			if !peer.Equal(p) {
				t.Fatal("querying chequebook for wrong peer")
			}
			return theirChequebookAddress, true, nil
		},
	}

	swapService := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		mockchequebook.NewChequebook(
			mockchequebook.WithChequebookAddressFunc(func() common.Address {
				return ourChequebookAddress
			}),
		),
		mockchequestore.NewChequeStore(),
		addressbook,
		uint64(1),
		&cashoutMock{
			cashCheque: func(ctx context.Context, c common.Address, r common.Address) (common.Hash, error) {
				if c != theirChequebookAddress {
					t.Fatalf("not cashing with the right chequebook. wanted %v, got %v", theirChequebookAddress, c)
				}
				if r != ourChequebookAddress {
					t.Fatalf("not cashing with the right recipient. wanted %v, got %v", ourChequebookAddress, r)
				}
				return txHash, nil
			},
		},
		mockp2p.New(),
	)

	returnedHash, err := swapService.CashCheque(context.Background(), peer)
	if err != nil {
		t.Fatal(err)
	}

	if returnedHash != txHash {
		t.Fatalf("go wrong tx hash. wanted %v, got %v", txHash, returnedHash)
	}
}

func TestCashoutStatus(t *testing.T) {
	logger := logging.New(ioutil.Discard, 0)
	store := mockstore.NewStateStore()

	theirChequebookAddress := common.HexToAddress("ffff")
	peer := infinity.MustParseHexAddress("abcd")
	addressbook := &addressbookMock{
		chequebook: func(p infinity.Address) (common.Address, bool, error) {
			if !peer.Equal(p) {
				t.Fatal("querying chequebook for wrong peer")
			}
			return theirChequebookAddress, true, nil
		},
	}

	expectedStatus := &chequebook.CashoutStatus{}

	swapService := swap.New(
		&swapProtocolMock{},
		logger,
		store,
		mockchequebook.NewChequebook(),
		mockchequestore.NewChequeStore(),
		addressbook,
		uint64(1),
		&cashoutMock{
			cashoutStatus: func(ctx context.Context, c common.Address) (*chequebook.CashoutStatus, error) {
				if c != theirChequebookAddress {
					t.Fatalf("getting status for wrong chequebook. wanted %v, got %v", theirChequebookAddress, c)
				}
				return expectedStatus, nil
			},
		},
		mockp2p.New(),
	)

	returnedStatus, err := swapService.CashoutStatus(context.Background(), peer)
	if err != nil {
		t.Fatal(err)
	}

	if expectedStatus != returnedStatus {
		t.Fatalf("go wrong status. wanted %v, got %v", expectedStatus, returnedStatus)
	}
}
