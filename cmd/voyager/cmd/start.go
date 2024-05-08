// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"errors"
	"fmt"

	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/external"
	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	// "github.com/yanhuangpai/voyager/hpc"

	"github.com/yanhuangpai/voyager"

	"github.com/yanhuangpai/voyager/pkg/cpc"
	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/keystore"
	filekeystore "github.com/yanhuangpai/voyager/pkg/keystore/file"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/node"

	"github.com/yanhuangpai/voyager/pkg/resolver/multiresolver"
)

const (
	serviceName = "InfinityVoyagerSvc"
)

func (c *command) initStartCmd() (err error) {
	cmd := &cobra.Command{
		Use:   "",
		Short: "",
	}
	c.start(cmd)
	// c.setAllFlags(cmd)
	// c.root.AddCommand(cmd)
	return nil
}

func (c *command) start(cmd *cobra.Command) (err error) {

	var logger logging.Logger
	switch v := strings.ToLower("info"); v {
	case "0", "silent":
		logger = logging.New(cmd.OutOrStdout(), 0)
	case "1", "error":
		logger = logging.New(cmd.OutOrStdout(), logrus.ErrorLevel)
	case "2", "warn":
		logger = logging.New(cmd.OutOrStdout(), logrus.WarnLevel)
	case "3", "info":
		logger = logging.New(cmd.OutOrStdout(), logrus.InfoLevel)
	case "4", "debug":
		logger = logging.New(cmd.OutOrStdout(), logrus.DebugLevel)
	case "5", "trace":
		logger = logging.New(cmd.OutOrStdout(), logrus.TraceLevel)
	default:
		return fmt.Errorf("unknown verbosity level %q", v)
	}

	isWindowsService, err := isWindowsService()
	if err != nil {
		return fmt.Errorf("failed to determine if we are running in service: %w", err)
	}

	if isWindowsService {
		var err error
		logger, err = createWindowsEventLogger(serviceName, logger)
		if err != nil {
			return fmt.Errorf("failed to create windows logger %w", err)
		}
	}

	// If the resolver is specified, resolve all connection strings
	// and fail on any errors.
	var resolverCfgs []multiresolver.ConnectionConfig
	resolverEndpoints := GetStringSlice("")
	if len(resolverEndpoints) > 0 {
		resolverCfgs, err = multiresolver.ParseConnectionStrings(resolverEndpoints)
		if err != nil {
			return err
		}
	}

	fmt.Println("============================================================")
	banner := `   _____                      __     _____ __          _     
  / ___/____ ___  ___   _____/ /_   / ____/ /_  ____  (_)___ 
  \__ \/ __ ` + "/" + `__ \/ __ ` + "\\" + `/ ___/  __/ / /   / __ \/ __ ` + "|" + `/ / __ \
 ___/ / / / / / / /_/ / /  / /_   / /___/ / / / /_/ / / / / /
/____/_/ /_/ /_/\__,_/_/   \__/   \____/_/ /_/\__,_/_/_/ /_/ 
`
	fmt.Println(banner)
	fmt.Println("============================================================")

	exist := cpc.CheckService()
	if exist {
		fmt.Println("Err:An existing process is running!")
		return nil
	}
	conf := cpc.GetConfig()
	flg := cpc.InterruptFlag{
		BalanceCheck: false,
		VersionCheck: false,
	}
	cpc.VersionCheck(&flg)
	logger.Info("Current Version:", conf.Version)

	logger.Infof("version: %v", voyager.Version)

	newOption := getNewOption("", logger, resolverCfgs)

	// fmt.Println("调试专用,取消DeviceHardWareCheck")
	passed, msg := cpc.DeviceHardWareCheck()
	if !passed {
		fmt.Println(msg)
		return nil
	}

	signerConfig, err := c.configureSigner(logger, *newOption)
	if err != nil {
		return err
	}

	b, _, ownerAddress, err := node.NewVoyager(
		newOption.Addr,
		signerConfig.OwnerAddress,
		signerConfig.DeviceInfo.Token,
		signerConfig.DeviceInfo.TestAddr,
		signerConfig.DeviceInfo.DeviceMac,
		signerConfig.Address,
		signerConfig.PublicKey,
		signerConfig.Signer,

		newOption.NetworkID,
		logger,
		signerConfig.Libp2pPrivateKey,
		signerConfig.PssPrivateKey,
		*newOption,
		&flg)
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("ownerAddress:", ownerAddress.String())

	if ownerAddress != nil {
		cpc.GetHPC(signerConfig, &flg)
	} else {
		logger.Infof("Swap is not Enabled!")
	}

	// fmt.Println("Happy Sharing......")

	go cpc.MessageOutput(&flg)

	// Wait for termination or interrupt signals.
	// We want to clean up things at the end.
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, syscall.SIGINT, syscall.SIGTERM)

	p := &program{
		start: func() {
			// Block main goroutine until it is interrupted
			sig := <-interruptChannel

			logger.Debugf("received signal: %v", sig)
			logger.Info("shutting down")
		},
		stop: func() {
			// Shutdown
			done := make(chan struct{})
			go func() {
				defer close(done)

				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				if err := b.Shutdown(ctx); err != nil {
					logger.Errorf("shutdown: %v", err)
				}
			}()

			// If shutdown function is blocking too long,
			// allow process termination by receiving another signal.
			select {
			case sig := <-interruptChannel:
				logger.Debugf("received signal: %v", sig)
			case <-done:
			}
		},
	}

	if isWindowsService {
		s, err := service.New(p, &service.Config{
			Name:        serviceName,
			DisplayName: "Voyager",
			Description: "Voyager, Smart Chain client.",
		})
		if err != nil {
			return err
		}

		if err = s.Run(); err != nil {
			return err
		}
	} else {
		// start blocks until some interrupt is received
		p.start()
		p.stop()
	}

	return nil
}

type program struct {
	start func()
	stop  func()
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.start()
	return nil
}

func (p *program) Stop(s service.Service) error {
	p.stop()
	return nil
}

func waitForClef(logger logging.Logger, maxRetries uint64, endpoint string) (externalSigner *external.ExternalSigner, err error) {
	for {
		externalSigner, err = external.NewExternalSigner(endpoint)
		if err == nil {
			return externalSigner, nil
		}
		if maxRetries == 0 {
			return nil, err
		}
		maxRetries--
		logger.Warningf("failing to connect to clef signer: %v", err)

		time.Sleep(5 * time.Second)
	}
}

func (c *command) configureSigner(logger logging.Logger, option node.Options) (config *cpc.SignerConfig, err error) {
	var (
		keystore keystore.Service
		// signer   crypto.Signer
		// address           infinity.Address
		password string
		// publicKey         *ecdsa.PublicKey
		overlayEthAddress string
		signerInfo        *cpc.SignerInfo
	)
	// conf := cpc.GetConfig()
	keystore = filekeystore.New(filepath.Join("./", "keys"))
	if p := option.Password; p != "" {
		// 设备码不为空
		password = p
	} else {
		// if libp2p key exists we can assume all required keys exist
		// so prompt for a password to unlock them
		// otherwise prompt for new password with confirmation to create them
		exists, err := keystore.Exists("libp2p")
		if err != nil {
			return nil, err
		}
		if exists {
			password, err = terminalPromptPassword(c.passwordReader, "Password")
			if err != nil {
				return nil, err
			}
		}
	}

	deviceInfo, err := cpc.GetDeviceInfoFromSrv()
	if err != nil {
		// fmt.Println(err)
		return nil, err
	}

	if fileExists("./keys") {
		// 文件存在，已生成主网地址，比对本地与服务器存储设备的信息

		signerInfo, err = GetSignerInfo(password, keystore, option.NetworkID)
		if err != nil {
			fmt.Println("Err:", err)
			return nil, err
		}
		overlayEthAddress = signerInfo.OverlayEthAddress.String()
		// 对主网进行地址比

		if !strings.EqualFold(deviceInfo.Addr, overlayEthAddress) {

			fmt.Println("Overlay EthAddress:", overlayEthAddress)
			fmt.Println("Registered address:", deviceInfo.Addr)
			fmt.Println("The OwnerAddress does not match")
			return nil, errors.New("The OwnerAddress does not match")
		}
	} else {
		// 文件不存在，主网地址还未生成，用户输入测试网地址,并进行绑定

		signerInfo, err = GetSignerInfo(password, keystore, option.NetworkID)
		if err != nil {
			return nil, err
		}
		overlayEthAddress = signerInfo.OverlayEthAddress.String()
		deviceInfo.TestAddr = overlayEthAddress
	}

	logger.Infof("Smart Chain public key %x", crypto.EncodeSecp256k1PublicKey(signerInfo.PublicKey))

	libp2pPrivateKey, _, err := keystore.Key("libp2p", password)
	if err != nil {
		return nil, fmt.Errorf("libp2p key: %w", err)
	}

	pssPrivateKey, created, err := keystore.Key("pss", password)
	if err != nil {
		return nil, fmt.Errorf("pss key: %w", err)
	}
	if created {
		logger.Debugf("new pss key created")
	} else {
		logger.Debugf("using existing pss key")
	}

	logger.Infof("pss public key %x", crypto.EncodeSecp256k1PublicKey(&pssPrivateKey.PublicKey))

	logger.Infof("using Smart Chain address %s", overlayEthAddress)

	return &cpc.SignerConfig{
		Signer:           signerInfo.Signer,
		Address:          signerInfo.Address,
		PublicKey:        signerInfo.PublicKey,
		Libp2pPrivateKey: libp2pPrivateKey,
		PssPrivateKey:    pssPrivateKey,
		DeviceInfo:       deviceInfo,
		// TestnetAddr:      deviceInfo.TestAddr,
		// Token:            deviceInfo.Token,
		// TureMac:          deviceInfo.DeviceMac,
		OwnerAddress: overlayEthAddress,
	}, nil

}

func getNewOption(debugAPIAddr string, logger logging.Logger, resolverCfgs []multiresolver.ConnectionConfig) *node.Options {
	conf := cpc.GetConfig()
	return &node.Options{
		DataDir:                   "./",
		DBCapacity:                5000000,
		DBOpenFilesLimit:          200,
		DBBlockCacheCapacity:      33554432,
		DBWriteBufferSize:         33554432,
		DBDisableSeeksCompaction:  false,
		APIAddr:                   "127.0.0.1:11633",
		DebugAPIAddr:              ":1645",
		Addr:                      ":11635",
		NATAddr:                   "54.252.195.103:11634",
		EnableWS:                  true,
		EnableQUIC:                true,
		WelcomeMessage:            "Welcome to Voygaer",
		Bootnodes:                 GetStringSlice("/ip4/54.252.195.103/tcp/11634/p2p/4c3948a814c430d3be4768e96a6c461f9223c0a0c47ac531df2c3e117639e28b3dc07ebfa36f5c2e718520e3b23561ba3cdf4de5f51b925eb9f139b4c80b1656"),
		CORSAllowedOrigins:        GetStringSlice("*"),
		Standalone:                false,
		TracingEnabled:            false,
		TracingEndpoint:           "127.0.0.1:6831",
		TracingServiceName:        "fish",
		Logger:                    logger,
		GlobalPinningEnabled:      true,
		PaymentThreshold:          "10000000000000",
		PaymentTolerance:          "50000000000000",
		PaymentEarly:              "1000000000000",
		ResolverConnectionCfgs:    resolverCfgs,
		GatewayMode:               true,
		BootnodeMode:              true,
		SwapEndpoint:              "http://52.77.248.72:18545",
		SwapFactoryAddress:        "0x7edFFD0a5422d4A9241DB77633CAfba8b578bE75",
		SwapInitialDeposit:        "0",
		SwapEnable:                true,
		Password:                  conf.IdKey,
		ClefSignerEnable:          false,
		ClefSignerEndpoint:        "",
		ClefSignerEthereumAddress: "",
		NetworkID:                 16688,
		LogicalCores:              4,
		MHZ:                       1.8,
		TotalFree:                 500,
	}
}

func GetStringSlice(str string) []string {
	var strSlice []string

	strSlice = make([]string, 0)
	strSlice = []string{str}
	return strSlice
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func GetSignerInfo(password string, keystore keystore.Service, NetworkID uint64) (*cpc.SignerInfo, error) {

	// infinityPrivateKey, _, err := keystore.Key("smartchain", strings.ToLower(password))
	infinityPrivateKey, _, err := keystore.Key("smartchain", password)
	if err != nil {
		return nil, fmt.Errorf("smart chain key: %w", err)
	}
	signer := crypto.NewDefaultSigner(infinityPrivateKey)
	publicKey := &infinityPrivateKey.PublicKey
	address, err := crypto.NewOverlayAddress(*publicKey, NetworkID)
	if err != nil {
		return nil, err
	}
	// postinst and post scripts inside packaging/{deb,rpm} depend and parse on this log output
	overlayEthAddress, err := signer.EthereumAddress()
	if err != nil {
		return nil, err
	}
	return &cpc.SignerInfo{
		InfinityPrivateKey: infinityPrivateKey,
		Signer:             signer,
		PublicKey:          publicKey,
		Address:            address,
		OverlayEthAddress:  overlayEthAddress,
	}, nil
}
