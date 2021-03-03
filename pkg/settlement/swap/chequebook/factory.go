// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chequebook

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethersphere/bee/pkg/settlement/swap/transaction"
	"github.com/ethersphere/sw3-bindings/v3/simpleswapfactory"
	"golang.org/x/net/context"
)

var (
	ErrInvalidFactory       = errors.New("not a valid factory contract")
	ErrNotDeployedByFactory = errors.New("chequebook not deployed by factory")

	factoryABI                  = transaction.ParseABIUnchecked(simpleswapfactory.SimpleSwapFactoryABI)
	simpleSwapDeployedEventType = factoryABI.Events["SimpleSwapDeployed"]
)

// Factory is the main interface for interacting with the chequebook factory.
type Factory interface {
	// ERC20Address returns the token for which this factory deploys chequebooks.
	ERC20Address(ctx context.Context) (common.Address, error)
	// Deploy deploys a new chequebook and returns once the transaction has been submitted.
	Deploy(ctx context.Context, issuer common.Address, defaultHardDepositTimeoutDuration *big.Int) (common.Hash, error)
	// WaitDeployed waits for the deployment transaction to confirm and returns the chequebook address
	WaitDeployed(ctx context.Context, txHash common.Hash) (common.Address, error)
	// VerifyBytecode checks that the factory is valid.
	VerifyBytecode(ctx context.Context) error
	// VerifyChequebook checks that the supplied chequebook has been deployed by this factory.
	VerifyChequebook(ctx context.Context, chequebook common.Address) error
}

type factory struct {
	backend            transaction.Backend
	transactionService transaction.Service
	address            common.Address

	instance SimpleSwapFactoryBinding
}

type simpleSwapDeployedEvent struct {
	ContractAddress common.Address
}

// NewFactory creates a new factory service for the provided factory contract.
func NewFactory(backend transaction.Backend, transactionService transaction.Service, address common.Address, simpleSwapFactoryBindingFunc SimpleSwapFactoryBindingFunc) (Factory, error) {
	instance, err := simpleSwapFactoryBindingFunc(address, backend)
	if err != nil {
		return nil, err
	}

	return &factory{
		backend:            backend,
		transactionService: transactionService,
		address:            address,
		instance:           instance,
	}, nil
}

// Deploy deploys a new chequebook and returns once the transaction has been submitted.
func (c *factory) Deploy(ctx context.Context, issuer common.Address, defaultHardDepositTimeoutDuration *big.Int) (common.Hash, error) {
	callData, err := factoryABI.Pack("deploySimpleSwap", issuer, big.NewInt(0).Set(defaultHardDepositTimeoutDuration))
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:       &c.address,
		Data:     callData,
		GasPrice: nil,
		GasLimit: 0,
		Value:    big.NewInt(0),
	}

	txHash, err := c.transactionService.Send(ctx, request)
	if err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}

// WaitDeployed waits for the deployment transaction to confirm and returns the chequebook address
func (c *factory) WaitDeployed(ctx context.Context, txHash common.Hash) (common.Address, error) {
	receipt, err := c.transactionService.WaitForReceipt(ctx, txHash)
	if err != nil {
		return common.Address{}, err
	}

	var event simpleSwapDeployedEvent
	err = transaction.FindSingleEvent(&factoryABI, receipt, c.address, simpleSwapDeployedEventType, &event)
	if err != nil {
		return common.Address{}, fmt.Errorf("contract deployment failed: %w", err)
	}

	return event.ContractAddress, nil
}

// VerifyBytecode checks that the factory is valid.
func (c *factory) VerifyBytecode(ctx context.Context) (err error) {
	code, err := c.backend.CodeAt(ctx, c.address, nil)
	if err != nil {
		return err
	}

	referenceCode := common.FromHex(simpleswapfactory.SimpleSwapFactoryDeployedCode)
	if !bytes.Equal(code, referenceCode) {
		return ErrInvalidFactory
	}
	return nil
}

// VerifyChequebook checks that the supplied chequebook has been deployed by this factory.
func (c *factory) VerifyChequebook(ctx context.Context, chequebook common.Address) error {
	deployed, err := c.instance.DeployedContracts(&bind.CallOpts{
		Context: ctx,
	}, chequebook)
	if err != nil {
		return err
	}
	if !deployed {
		return ErrNotDeployedByFactory
	}
	return nil
}

// ERC20Address returns the token for which this factory deploys chequebooks.
func (c *factory) ERC20Address(ctx context.Context) (common.Address, error) {
	erc20Address, err := c.instance.ERC20Address(&bind.CallOpts{
		Context: ctx,
	})
	if err != nil {
		return common.Address{}, err
	}
	return erc20Address, nil
}

// DiscoverFactoryAddress returns the canonical factory for this chainID
func DiscoverFactoryAddress(chainID int64) (common.Address, bool) {
	if chainID == 5 {
		// goerli
		return common.HexToAddress("0xf0277caffea72734853b834afc9892461ea18474"), true
	}
	return common.Address{}, false
}
