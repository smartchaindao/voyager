// Copyright 2021 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/yanhuangpai/voyager/pkg/cpc"
	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/p2p/libp2p"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap/chequebook"

	"github.com/yanhuangpai/voyager/pkg/settlement/swap/swapprotocol"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap/transaction"
	"github.com/yanhuangpai/voyager/pkg/storage"
)

const (
	maxDelay = 1 * time.Minute
)

// InitChain will initialize the Ethereum backend at the given endpoint and
// set up the Transacton Service to interact with it using the provided signer.
func InitChain(
	ctx context.Context,
	logger logging.Logger,
	stateStore storage.StateStorer,
	endpoint string,
	signer crypto.Signer,
) (*ethclient.Client, common.Address, int64, transaction.Service, error) {
	backend, err := ethclient.Dial(endpoint)
	if err != nil {
		return nil, common.Address{}, 0, nil, fmt.Errorf("dial eth client: %w", err)
	}

	chainID, err := backend.ChainID(ctx)
	if err != nil {
		logger.Infof("could not connect to backend at %v. In a swap-enabled network a working blockchain node (for goerli network in production) is required. Check your node or specify another node using --swap-endpoint.", endpoint)
		return nil, common.Address{}, 0, nil, fmt.Errorf("get chain id: %w", err)
	}

	transactionService, err := transaction.NewService(logger, backend, signer, stateStore, chainID)
	if err != nil {
		return nil, common.Address{}, 0, nil, fmt.Errorf("new transaction service: %w", err)
	}
	overlayEthAddress, err := signer.EthereumAddress()
	if err != nil {
		return nil, common.Address{}, 0, nil, fmt.Errorf("eth address: %w", err)
	}

	// Sync the with the given Ethereum backend:
	// isSynced, err := transaction.IsSynced(ctx, backend, maxDelay)
	// if err != nil {
	// 	return nil, common.Address{}, 0, nil, fmt.Errorf("is synced: %w", err)
	// }
	// if !isSynced {
	// 	logger.Infof("waiting to sync with the Smart Chain backend")
	// 	err := transaction.WaitSynced(ctx, backend, maxDelay)
	// 	if err != nil {
	// 		return nil, common.Address{}, 0, nil, fmt.Errorf("waiting backend sync: %w", err)
	// 	}
	// }
	return backend, overlayEthAddress, chainID.Int64(), transactionService, nil
}

// InitChequebookFactory will initialize the chequebook factory with the given
// chain backend.
func InitChequebookFactory(
	logger logging.Logger,
	backend *ethclient.Client,
	chainID int64,
	transactionService transaction.Service,
	factoryAddress string,
) (chequebook.Factory, error) {
	var addr common.Address
	if factoryAddress == "" {
		var found bool
		addr, found = chequebook.DiscoverFactoryAddress(chainID)
		if !found {
			return nil, errors.New("no known factory address for this network")
		}
		logger.Infof("using default factory address for chain id %d: %x", chainID, addr)
	} else if !common.IsHexAddress(factoryAddress) {
		return nil, errors.New("malformed factory address")
	} else {
		addr = common.HexToAddress(factoryAddress)
		logger.Infof("using custom factory address: %x", addr)
	}

	return chequebook.NewFactory(
		backend,
		transactionService,
		addr,
	), nil
}

// InitCPUAwardService will initialize the cpuaward service with the given data

func InitCPUAwardService(
	overlayEthAddress common.Address,
	transactionService transaction.Service,
) (cpc.Service, error) {
	cpuAwardService, err := cpc.NewCPUAward(
		transactionService,
		overlayEthAddress,
	)
	if err != nil {
		return nil, fmt.Errorf("cpu award init: %w", err)
	}

	return cpuAwardService, nil
}

// InitChequebookService will initialize the chequebook service with the given
// chequebook factory and chain backend.
func InitChequebookService(
	ctx context.Context,
	logger logging.Logger,
	stateStore storage.StateStorer,
	signer crypto.Signer,
	chainID int64,
	backend *ethclient.Client,
	overlayEthAddress common.Address,
	transactionService transaction.Service,
	chequebookFactory chequebook.Factory,
	initialDeposit string,
) (chequebook.Service, error) {
	chequeSigner := chequebook.NewChequeSigner(signer, chainID)

	deposit, ok := new(big.Int).SetString(initialDeposit, 10)
	if !ok {
		return nil, fmt.Errorf("initial swap deposit \"%s\" cannot be parsed", initialDeposit)
	}

	chequebookService, err := chequebook.Init(
		ctx,
		chequebookFactory,
		stateStore,
		logger,
		deposit,
		transactionService,
		backend,
		chainID,
		overlayEthAddress,
		chequeSigner,
		chequebook.NewSimpleSwapBindings,
	)
	if err != nil {
		return nil, fmt.Errorf("chequebook init: %w", err)
	}

	return chequebookService, nil
}

func initChequeStoreCashout(
	stateStore storage.StateStorer,
	swapBackend transaction.Backend,
	chequebookFactory chequebook.Factory,
	chainID int64,
	overlayEthAddress common.Address,
	transactionService transaction.Service,
) (chequebook.ChequeStore, chequebook.CashoutService) {
	chequeStore := chequebook.NewChequeStore(
		stateStore,
		swapBackend,
		chequebookFactory,
		chainID,
		overlayEthAddress,
		chequebook.NewSimpleSwapBindings,
		chequebook.RecoverCheque,
	)

	cashout := chequebook.NewCashoutService(
		stateStore,
		swapBackend,
		transactionService,
		chequeStore,
	)

	return chequeStore, cashout
}

// InitSwap will initialize and register the swap service.
func InitSwap(
	p2ps *libp2p.Service,
	logger logging.Logger,
	stateStore storage.StateStorer,
	networkID uint64,
	overlayEthAddress common.Address,
	chequebookService chequebook.Service,
	chequeStore chequebook.ChequeStore,
	cashoutService chequebook.CashoutService,
) (*swap.Service, error) {
	swapProtocol := swapprotocol.New(p2ps, logger, overlayEthAddress)
	swapAddressBook := swap.NewAddressbook(stateStore)

	swapService := swap.New(
		swapProtocol,
		logger,
		stateStore,
		chequebookService,
		chequeStore,
		swapAddressBook,
		networkID,
		cashoutService,
		p2ps,
	)

	swapProtocol.SetSwap(swapService)

	err := p2ps.AddProtocol(swapProtocol.Protocol())
	if err != nil {
		return nil, err
	}

	return swapService, nil
}
