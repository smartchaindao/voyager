// Copyright 2021 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package node defines the concept of a Voyager node
// by bootstrapping and injecting all necessary
// dependencies.
package node

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
	"github.com/yanhuangpai/voyager/pkg/accounting"
	"github.com/yanhuangpai/voyager/pkg/addressbook"
	"github.com/yanhuangpai/voyager/pkg/api"
	"github.com/yanhuangpai/voyager/pkg/cpc"
	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/debugapi"
	"github.com/yanhuangpai/voyager/pkg/feeds/factory"
	"github.com/yanhuangpai/voyager/pkg/hive"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/kademlia"
	"github.com/yanhuangpai/voyager/pkg/localstore"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/metrics"
	"github.com/yanhuangpai/voyager/pkg/netstore"
	"github.com/yanhuangpai/voyager/pkg/p2p/libp2p"
	"github.com/yanhuangpai/voyager/pkg/pingpong"
	"github.com/yanhuangpai/voyager/pkg/pricing"
	"github.com/yanhuangpai/voyager/pkg/pss"
	"github.com/yanhuangpai/voyager/pkg/puller"
	"github.com/yanhuangpai/voyager/pkg/pullsync"
	"github.com/yanhuangpai/voyager/pkg/pullsync/pullstorage"
	"github.com/yanhuangpai/voyager/pkg/pusher"
	"github.com/yanhuangpai/voyager/pkg/pushsync"
	"github.com/yanhuangpai/voyager/pkg/recovery"
	"github.com/yanhuangpai/voyager/pkg/resolver/multiresolver"
	"github.com/yanhuangpai/voyager/pkg/retrieval"
	settlement "github.com/yanhuangpai/voyager/pkg/settlement"
	"github.com/yanhuangpai/voyager/pkg/settlement/pseudosettle"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap/chequebook"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap/transaction"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"github.com/yanhuangpai/voyager/pkg/tags"
	"github.com/yanhuangpai/voyager/pkg/tracing"
	"github.com/yanhuangpai/voyager/pkg/traversal"
	"golang.org/x/sync/errgroup"
)

type Voyager struct {
	p2pService            io.Closer
	p2pCancel             context.CancelFunc
	apiCloser             io.Closer
	apiServer             *http.Server
	debugAPIServer        *http.Server
	resolverCloser        io.Closer
	errorLogWriter        *io.PipeWriter
	tracerCloser          io.Closer
	tagsCloser            io.Closer
	stateStoreCloser      io.Closer
	localstoreCloser      io.Closer
	topologyCloser        io.Closer
	pusherCloser          io.Closer
	pullerCloser          io.Closer
	pullSyncCloser        io.Closer
	pssCloser             io.Closer
	ethClientCloser       func()
	recoveryHandleCleanup func()
}

type Options struct {
	DataDir                   string
	DBCapacity                uint64
	DBOpenFilesLimit          uint64
	DBWriteBufferSize         uint64
	DBBlockCacheCapacity      uint64
	DBDisableSeeksCompaction  bool
	APIAddr                   string
	DebugAPIAddr              string
	Addr                      string
	NATAddr                   string
	EnableWS                  bool
	EnableQUIC                bool
	WelcomeMessage            string
	Bootnodes                 []string
	CORSAllowedOrigins        []string
	Logger                    logging.Logger
	Standalone                bool
	TracingEnabled            bool
	TracingEndpoint           string
	TracingServiceName        string
	GlobalPinningEnabled      bool
	PaymentThreshold          string
	PaymentTolerance          string
	PaymentEarly              string
	ResolverConnectionCfgs    []multiresolver.ConnectionConfig
	GatewayMode               bool
	BootnodeMode              bool
	SwapEndpoint              string
	SwapFactoryAddress        string
	SwapInitialDeposit        string
	SwapEnable                bool
	Password                  string
	ClefSignerEnable          bool
	ClefSignerEndpoint        string
	ClefSignerEthereumAddress string
	NetworkID                 uint64
	LogicalCores              int
	MHZ                       float64
	TotalFree                 uint64
}

type Chequebook struct {
	Service        chequebook.Service
	Store          chequebook.ChequeStore
	CashoutService chequebook.CashoutService
}

type Services struct {
	debugAPIService   *debugapi.Service
	p2ps              *libp2p.Service
	pingPong          *pingpong.Service
	retrieve          *retrieval.Service
	pushSyncPusher    *pusher.Service
	pssService        pss.Interface
	apiService        api.Service
	swapService       *swap.Service
	chequebookService chequebook.Service
	tagService        *tags.Tags
	pullSync          *pullsync.Syncer
	puller            *puller.Puller
}

func NewVoyager(
	addr, owner_Address, token, testnetAddr, mac string,
	infinityAddress infinity.Address,
	publicKey *ecdsa.PublicKey,
	signer crypto.Signer,
	networkID uint64,
	logger logging.Logger,
	libp2pPrivateKey,
	pssPrivateKey *ecdsa.PrivateKey,
	op Options, flg *cpc.InterruptFlag) (voyager *Voyager, cpuawardService cpc.Service, ownerAddress *common.Address, err error) {
	var (
		services          Services
		swapBackend       *ethclient.Client
		overlayEthAddress common.Address
		chequebookService chequebook.Service
		chequeStore       chequebook.ChequeStore
		cashoutService    chequebook.CashoutService
		chequebooker      *Chequebook
		debugAPIService   *debugapi.Service
		settlement        settlement.Interface
		bootnodes         []ma.Multiaddr
		swapService       *swap.Service
		ns                storage.Storer
		path              string
	)

	tracer, tracerCloser, err := tracing.NewTracer(&tracing.Options{
		Enabled:     op.TracingEnabled,
		Endpoint:    op.TracingEndpoint,
		ServiceName: op.TracingServiceName,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tracer: %w", err)
	}
	p2pCtx, p2pCancel := context.WithCancel(context.Background())
	defer func() {
		// if there's voyagern an error on this function
		// we'd like to cancel the p2p context so that
		// incoming connections will not be possible
		if err != nil {
			p2pCancel()
		}
	}()
	voyager = &Voyager{
		p2pCancel:      p2pCancel,
		errorLogWriter: logger.WriterLevel(logrus.ErrorLevel),
		tracerCloser:   tracerCloser,
	}
	overlayEthAddress, err = signer.EthereumAddress()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("eth address: %w", err)
	}
	if token != "" {
		err = cpc.RegisterNode(testnetAddr, token, mac, owner_Address, infinityAddress, publicKey)
		if err != nil {
			fmt.Println("err", err)
			return nil, nil, nil, err
		} else {
			fmt.Println("Node registration succeeded!")
		}
	}
	if op.DebugAPIAddr != "" {

		// set up basic debug api endpoints for debugging and /health endpoint
		debugAPIService = debugapi.New(infinityAddress, *publicKey, pssPrivateKey.PublicKey, overlayEthAddress, logger, tracer, op.CORSAllowedOrigins)
		services.debugAPIService = debugAPIService
		debugAPIListener, err := net.Listen("tcp", op.DebugAPIAddr)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("debug api listener: %w", err)
		}

		debugAPIServer := &http.Server{
			IdleTimeout:       30 * time.Second,
			ReadHeaderTimeout: 3 * time.Second,
			Handler:           debugAPIService,
			ErrorLog:          log.New(voyager.errorLogWriter, "", 0),
		}
		go func() {
			// logger.Infof("debug api address: %s", debugAPIListener.Addr())

			if err := debugAPIServer.Serve(debugAPIListener); err != nil && err != http.ErrServerClosed {
				logger.Debugf("debug api server: %v", err)
				logger.Error("unable to serve debug api")
			}
		}()

		voyager.debugAPIServer = debugAPIServer
	}
	stateStore, err := InitStateStore(logger, op.DataDir)
	if err != nil {
		return nil, nil, nil, err
	}
	voyager.stateStoreCloser = stateStore
	err = CheckOverlayWithStore(infinityAddress, stateStore)
	if err != nil {
		fmt.Println(err)
		return nil, nil, nil, err
	}
	addressbook := addressbook.New(stateStore)

	p2ps, err := libp2p.New(p2pCtx, signer, networkID, infinityAddress, addr, addressbook, stateStore, logger, tracer, libp2p.Options{
		PrivateKey:     libp2pPrivateKey,
		NATAddr:        op.NATAddr,
		EnableWS:       op.EnableWS,
		EnableQUIC:     op.EnableQUIC,
		Standalone:     op.Standalone,
		WelcomeMessage: op.WelcomeMessage,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("p2p service: %w", err)
	}
	services.p2ps = p2ps

	voyager.p2pService = p2ps
	if op.SwapEnable {
		swapBackend, cpuawardService, chequebooker, ownerAddress, err = EnableSwap(p2pCtx, logger, stateStore, op, signer)
		chequeStore = chequebooker.Store
		cashoutService = chequebooker.CashoutService
		chequebookService = chequebooker.Service
		voyager.ethClientCloser = swapBackend.Close
		swapService, err = InitSwap(
			p2ps,
			logger,
			stateStore,
			networkID,
			overlayEthAddress,
			chequebookService,
			chequeStore,
			cashoutService,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		settlement = swapService
	} else {
		pseudosettleService := pseudosettle.New(p2ps, logger, stateStore)
		if err = p2ps.AddProtocol(pseudosettleService.Protocol()); err != nil {
			return nil, nil, nil, fmt.Errorf("pseudosettle service: %w", err)
		}
		settlement = pseudosettleService
	}
	if !op.Standalone {
		if natManager := p2ps.NATManager(); natManager != nil {
			// wait for nat manager to init
			logger.Debug("initializing NAT manager")
			select {
			case <-natManager.Ready():
				// this is magic sleep to give NAT time to sync the mappings
				// this is a hack, kind of alchemy and should be improved
				time.Sleep(3 * time.Second)
				logger.Debug("NAT manager initialized")
			case <-time.After(10 * time.Second):
				logger.Warning("NAT manager init timeout")
			}
		}
	}
	// Construct protocols.
	pingPong, hive, paymentThreshold, pricing, err := buildProtocols(p2ps, logger, tracer, addressbook, networkID, op)
	services.pingPong = pingPong
	if op.Standalone {
		logger.Info("Starting node in standalone mode, no p2p connections will be made or accepted")
	} else {
		for _, a := range op.Bootnodes {
			addr, err := ma.NewMultiaddr(a)
			if err != nil {
				// logger.Debugf("multiaddress fail %s: %v", a, err)
				// logger.Warningf("invalid bootnode address %s", a)
				continue
			}

			bootnodes = append(bootnodes, addr)
		}
	}

	paymentTolerance, ok := new(big.Int).SetString(op.PaymentTolerance, 10)
	if !ok {
		return nil, nil, nil, fmt.Errorf("invalid payment tolerance: %s", paymentTolerance)
	}
	paymentEarly, ok := new(big.Int).SetString(op.PaymentEarly, 10)
	if !ok {
		return nil, nil, nil, fmt.Errorf("invalid payment early: %s", paymentEarly)
	}
	acc, err := accounting.NewAccounting(
		paymentThreshold,
		paymentTolerance,
		paymentEarly,
		logger,
		stateStore,
		settlement,
		pricing,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("accounting: %w", err)
	}
	settlement.SetNotifyPaymentFunc(acc.AsyncNotifyPayment)
	pricing.SetPaymentThresholdObserver(acc)
	kad := kademlia.New(infinityAddress, addressbook, hive, p2ps, logger, kademlia.Options{Bootnodes: bootnodes, StandaloneMode: op.Standalone, BootnodeMode: op.BootnodeMode})
	voyager.topologyCloser = kad
	hive.SetAddPeersHandler(kad.AddPeers)
	p2ps.SetPickyNotifier(kad)
	addrs, err := p2ps.Addresses()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get server addresses: %w", err)
	}
	for _, addr := range addrs {
		logger.Debugf("p2p address: %s", addr)
	}

	if op.DataDir != "" {
		path = filepath.Join(op.DataDir, "localstore")
	}
	lo := &localstore.Options{
		Capacity:               op.DBCapacity,
		OpenFilesLimit:         op.DBOpenFilesLimit,
		BlockCacheCapacity:     op.DBBlockCacheCapacity,
		WriteBufferSize:        op.DBWriteBufferSize,
		DisableSeeksCompaction: op.DBDisableSeeksCompaction,
	}
	storer, err := localstore.New(path, infinityAddress.Bytes(), lo, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("localstore: %w", err)
	}
	voyager.localstoreCloser = storer
	retrieve := retrieval.New(infinityAddress, storer, p2ps, kad, logger, acc, accounting.NewFixedPricer(infinityAddress, 1000000000), tracer)
	services.retrieve = retrieve
	tagService := tags.NewTags(stateStore, logger)
	services.tagService = tagService
	voyager.tagsCloser = tagService

	if err = p2ps.AddProtocol(retrieve.Protocol()); err != nil {
		return nil, nil, nil, fmt.Errorf("retrieval service: %w", err)
	}
	pssService := pss.New(pssPrivateKey, logger)
	services.pssService = pssService
	voyager.pssCloser = pssService

	if op.GlobalPinningEnabled {
		// create recovery callback for content repair
		recoverFunc := recovery.NewCallback(pssService)
		ns = netstore.New(storer, recoverFunc, retrieve, logger)
	} else {
		ns = netstore.New(storer, nil, retrieve, logger)
	}

	traversalService := traversal.NewService(ns)

	pushSyncProtocol := pushsync.New(p2ps, storer, kad, tagService, pssService.TryUnwrap, logger, acc, accounting.NewFixedPricer(infinityAddress, 1000000000), tracer)

	// set the pushSyncer in the PSS
	pssService.SetPushSyncer(pushSyncProtocol)

	if err = p2ps.AddProtocol(pushSyncProtocol.Protocol()); err != nil {
		return nil, nil, nil, fmt.Errorf("pushsync service: %w", err)
	}

	if op.GlobalPinningEnabled {
		// register function for chunk repair upon receiving a trojan message
		chunkRepairHandler := recovery.NewRepairHandler(ns, logger, pushSyncProtocol)
		voyager.recoveryHandleCleanup = pssService.Register(recovery.Topic, chunkRepairHandler)
	}
	pushSyncPusher := pusher.New(storer, kad, pushSyncProtocol, tagService, logger, tracer)
	services.pushSyncPusher = pushSyncPusher
	voyager.pusherCloser = pushSyncPusher

	pullStorage := pullstorage.New(storer)

	pullSync := pullsync.New(p2ps, pullStorage, pssService.TryUnwrap, logger)
	services.pullSync = pullSync
	voyager.pullSyncCloser = pullSync

	if err = p2ps.AddProtocol(pullSync.Protocol()); err != nil {
		return nil, nil, nil, fmt.Errorf("pullsync protocol: %w", err)
	}

	puller := puller.New(stateStore, kad, pullSync, logger, puller.Options{})
	services.puller = puller
	voyager.pullerCloser = puller

	multiResolver := multiresolver.NewMultiResolver(
		multiresolver.WithConnectionConfigs(op.ResolverConnectionCfgs),
		multiresolver.WithLogger(op.Logger),
	)
	voyager.resolverCloser = multiResolver
	if op.APIAddr != "" {
		apiServer, apiService := APIServer(ns, tagService, multiResolver, pssService, traversalService, logger, tracer, op, *voyager, flg)
		voyager.apiServer = apiServer
		voyager.apiCloser = apiService
		services.apiService = apiService
	}

	if debugAPIService != nil {
		registerMetrics(services, acc, storer, pushSyncProtocol, logger, settlement, kad, op)
	}

	if err := kad.Start(p2pCtx); err != nil {
		return nil, nil, nil, err
	}
	p2ps.Ready()
	return voyager, cpuawardService, ownerAddress, nil
}

func (voyager *Voyager) Shutdown(ctx context.Context) error {
	errs := new(multiError)

	if voyager.apiCloser != nil {
		if err := voyager.apiCloser.Close(); err != nil {
			errs.add(fmt.Errorf("api: %w", err))
		}
	}

	var eg errgroup.Group
	if voyager.apiServer != nil {
		eg.Go(func() error {
			if err := voyager.apiServer.Shutdown(ctx); err != nil {
				return fmt.Errorf("api server: %w", err)
			}
			return nil
		})
	}
	if voyager.debugAPIServer != nil {
		eg.Go(func() error {
			if err := voyager.debugAPIServer.Shutdown(ctx); err != nil {
				return fmt.Errorf("debug api server: %w", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		errs.add(err)
	}

	if voyager.recoveryHandleCleanup != nil {
		voyager.recoveryHandleCleanup()
	}

	if err := voyager.pusherCloser.Close(); err != nil {
		errs.add(fmt.Errorf("pusher: %w", err))
	}

	if err := voyager.pullerCloser.Close(); err != nil {
		errs.add(fmt.Errorf("puller: %w", err))
	}

	if err := voyager.pullSyncCloser.Close(); err != nil {
		errs.add(fmt.Errorf("pull sync: %w", err))
	}

	if err := voyager.pssCloser.Close(); err != nil {
		errs.add(fmt.Errorf("pss: %w", err))
	}

	voyager.p2pCancel()
	if err := voyager.p2pService.Close(); err != nil {
		errs.add(fmt.Errorf("p2p server: %w", err))
	}

	if c := voyager.ethClientCloser; c != nil {
		c()
	}

	if err := voyager.tracerCloser.Close(); err != nil {
		errs.add(fmt.Errorf("tracer: %w", err))
	}

	if err := voyager.tagsCloser.Close(); err != nil {
		errs.add(fmt.Errorf("tag persistence: %w", err))
	}

	if err := voyager.stateStoreCloser.Close(); err != nil {
		errs.add(fmt.Errorf("statestore: %w", err))
	}

	if err := voyager.localstoreCloser.Close(); err != nil {
		errs.add(fmt.Errorf("localstore: %w", err))
	}

	if err := voyager.topologyCloser.Close(); err != nil {
		errs.add(fmt.Errorf("topology driver: %w", err))
	}

	if err := voyager.errorLogWriter.Close(); err != nil {
		errs.add(fmt.Errorf("error log writer: %w", err))
	}

	// Shutdown the resolver service only if it has voyagern initialized.
	if voyager.resolverCloser != nil {
		if err := voyager.resolverCloser.Close(); err != nil {
			errs.add(fmt.Errorf("resolver service: %w", err))
		}
	}

	if errs.hasErrors() {
		return errs
	}

	return nil
}

type multiError struct {
	errors []error
}

func (e *multiError) Error() string {
	if len(e.errors) == 0 {
		return ""
	}
	s := e.errors[0].Error()
	for _, err := range e.errors[1:] {
		s += "; " + err.Error()
	}
	return s
}

func (e *multiError) add(err error) {
	e.errors = append(e.errors, err)
}

func (e *multiError) hasErrors() bool {
	return len(e.errors) > 0
}

func EnableSwap(p2pCtx context.Context, logger logging.Logger, stateStore storage.StateStorer, op Options, signer crypto.Signer) (*ethclient.Client, cpc.Service, *Chequebook, *common.Address, error) {
	var (
		swapBackend        *ethclient.Client
		chainID            int64
		transactionService transaction.Service
		chequebookFactory  chequebook.Factory
	)
	swapBackend, overlayEthAddress, chainID, transactionService, err := InitChain(
		p2pCtx,
		logger,
		stateStore,
		op.SwapEndpoint,
		signer,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	chequebookFactory, err = InitChequebookFactory(
		logger,
		swapBackend,
		chainID,
		transactionService,
		op.SwapFactoryAddress,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if err = chequebookFactory.VerifyBytecode(p2pCtx); err != nil {
		// return fmt.Errorf("factory fail: %w", err)
		return nil, nil, nil, nil, err
	}

	cpuawardService, err := InitCPUAwardService(
		overlayEthAddress,
		transactionService,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	chequeStore, cashoutService := initChequeStoreCashout(
		stateStore,
		swapBackend,
		chequebookFactory,
		chainID,
		overlayEthAddress,
		transactionService,
	)
	chequebook := Chequebook{
		// Service:        chequebookService,
		Service:        nil,
		Store:          chequeStore,
		CashoutService: cashoutService,
	}
	return swapBackend, cpuawardService, &chequebook, &overlayEthAddress, nil

}

func buildProtocols(p2ps *libp2p.Service, logger logging.Logger, tracer *tracing.Tracer, addressbook addressbook.Interface, networkID uint64, op Options) (*pingpong.Service, *hive.Service, *big.Int, *pricing.Service, error) {
	pingPong := pingpong.New(p2ps, logger, tracer)
	var err error
	if err = p2ps.AddProtocol(pingPong.Protocol()); err != nil {
		fmt.Errorf("pingpong service: %w", err)
		return nil, nil, nil, nil, err
	}

	hive := hive.New(p2ps, addressbook, networkID, logger)
	if err = p2ps.AddProtocol(hive.Protocol()); err != nil {
		fmt.Errorf("hive service: %w", err)
		return nil, nil, nil, nil, err
	}

	paymentThreshold, ok := new(big.Int).SetString(op.PaymentThreshold, 10)
	if !ok {
		fmt.Errorf("invalid payment threshold: %s", paymentThreshold)
		return nil, nil, nil, nil, err
	}
	pricing := pricing.New(p2ps, logger, paymentThreshold)
	if err := p2ps.AddProtocol(pricing.Protocol()); err != nil {
		fmt.Errorf("pricing service: %w", err)
		return nil, nil, nil, nil, err
	}
	return pingPong, hive, paymentThreshold, pricing, nil
}

func APIServer(ns storage.Storer, tagService *tags.Tags, multiResolver *multiresolver.MultiResolver, pssService pss.Interface, traversalService traversal.Service, logger logging.Logger, tracer *tracing.Tracer, op Options, voyager Voyager, flg *cpc.InterruptFlag) (*http.Server, api.Service) {
	// API server
	feedFactory := factory.New(ns)
	apiService := api.New(tagService, ns, multiResolver, pssService, traversalService, feedFactory, logger, tracer, api.Options{
		CORSAllowedOrigins: op.CORSAllowedOrigins,
		GatewayMode:        op.GatewayMode,
		WsPingPeriod:       60 * time.Second,
	}, flg)
	apiListener, err := net.Listen("tcp", op.APIAddr)
	if err != nil {
		fmt.Errorf("api listener: %w", err)
		return nil, nil
	}

	apiServer := &http.Server{
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           apiService,
		ErrorLog:          log.New(voyager.errorLogWriter, "", 0),
	}

	go func() {
		// logger.Infof("api address: %s", apiListener.Addr())
		// logger.Info("apiServer.Addr:", apiServer.Addr)

		if err := apiServer.Serve(apiListener); err != nil && err != http.ErrServerClosed {
			logger.Debugf("api server: %v", err)
			logger.Error("unable to serve api")
		}
	}()
	return apiServer, apiService
}

func registerMetrics(
	services Services,
	acc *accounting.Accounting,
	storer *localstore.DB,
	pushSyncProtocol *pushsync.PushSync,
	logger logging.Logger,
	settlement settlement.Interface,
	kad *kademlia.Kad,
	op Options,
) {
	debugAPIService := services.debugAPIService
	// register metrics from components
	debugAPIService.MustRegisterMetrics(services.p2ps.Metrics()...)
	debugAPIService.MustRegisterMetrics(services.pingPong.Metrics()...)
	debugAPIService.MustRegisterMetrics(acc.Metrics()...)
	debugAPIService.MustRegisterMetrics(storer.Metrics()...)
	debugAPIService.MustRegisterMetrics(services.puller.Metrics()...)
	debugAPIService.MustRegisterMetrics(pushSyncProtocol.Metrics()...)
	debugAPIService.MustRegisterMetrics(services.pushSyncPusher.Metrics()...)
	debugAPIService.MustRegisterMetrics(services.pullSync.Metrics()...)
	debugAPIService.MustRegisterMetrics(services.retrieve.Metrics()...)

	if pssServiceMetrics, ok := services.pssService.(metrics.Collector); ok {
		debugAPIService.MustRegisterMetrics(pssServiceMetrics.Metrics()...)
	}

	if services.apiService != nil {
		debugAPIService.MustRegisterMetrics(services.apiService.Metrics()...)
	}
	if l, ok := logger.(metrics.Collector); ok {
		debugAPIService.MustRegisterMetrics(l.Metrics()...)
	}

	if l, ok := settlement.(metrics.Collector); ok {
		debugAPIService.MustRegisterMetrics(l.Metrics()...)
	}

	// inject dependencies and configure full debug api http path routes
	debugAPIService.Configure(services.p2ps, services.pingPong, kad, storer, services.tagService, acc, settlement, op.SwapEnable, services.swapService, services.chequebookService)
}
