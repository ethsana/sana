// Copyright 2021 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package minercontract

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethersphere/bee/pkg/mine"
	"github.com/ethersphere/bee/pkg/sctx"
	"github.com/ethersphere/bee/pkg/transaction"
)

var (
	minerABI       = transaction.ParseABIUnchecked(minerAbi)
	minerEventType = minerABI.Events["Miner"]
	errDecodeABI   = errors.New("could not decode abi data")
)

type MinerEvent struct {
	Node       common.Hash
	Chequebook common.Address
}

type minerService struct {
	backend            transaction.Backend
	transactionService transaction.Service
	address            common.Address
}

func New(backend transaction.Backend, transactionService transaction.Service, address common.Address) mine.MinerContract {
	return &minerService{
		backend:            backend,
		transactionService: transactionService,
		address:            address,
	}
}

func (svc *minerService) Withdraw(ctx context.Context, node common.Hash, deadline *big.Int, sign []byte) (common.Hash, error) {
	callData, err := minerABI.Pack(`withdraw`, node, deadline, sign)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &svc.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		GasLimit:    9000000,
		Value:       big.NewInt(0),
		Description: "withdraw ",
	}

	txHash, err := svc.transactionService.Send(ctx, request)
	if err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}

func (svc *minerService) Reward(ctx context.Context, node common.Hash) (*big.Int, error) {
	callData, err := minerABI.Pack(`reward`, node)
	if err != nil {
		return nil, err
	}

	output, err := svc.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &svc.address,
		Data: callData,
	})
	if err != nil {
		return nil, err
	}

	results, err := minerABI.Unpack("reward", output)
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, errDecodeABI
	}

	balance, ok := abi.ConvertType(results[0], new(big.Int)).(*big.Int)
	if !ok || balance == nil {
		return nil, errDecodeABI
	}
	return balance, nil
}

func (svc *minerService) IsMiner(ctx context.Context, node common.Hash) (bool, error) {
	callData, err := minerABI.Pack(`miners`, node)
	if err != nil {
		return false, err
	}

	output, err := svc.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &svc.address,
		Data: callData,
	})
	if err != nil {
		return false, err
	}

	results, err := minerABI.Unpack("miners", output)
	if err != nil {
		return false, err
	}
	if len(results) != 5 {
		return false, errDecodeABI
	}
	return results[0].(bool), nil
}

func (svc *minerService) Regist(ctx context.Context, node common.Hash, chequebook common.Address) (common.Hash, error) {
	callData, err := minerABI.Pack(`regist`, node, chequebook)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &svc.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		GasLimit:    200000,
		Value:       big.NewInt(0),
		Description: "regist ",
	}

	txHash, err := svc.transactionService.Send(ctx, request)
	if err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}

func (s *minerService) WaitRegist(ctx context.Context, txHash common.Hash) (uint64, error) {
	receipt, err := s.transactionService.WaitForReceipt(ctx, txHash)
	if err != nil {
		return 0, err
	}

	return receipt.Status, nil
}
