// Copyright 2021 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package minecontract

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/mine"
	"github.com/ethsana/sana/pkg/sctx"
	"github.com/ethsana/sana/pkg/transaction"
)

var (
	mineABI      = transaction.ParseABIUnchecked(MineABI)
	lockupABI    = transaction.ParseABIUnchecked(LockupABI)
	errDecodeABI = errors.New("could not decode abi data")
)

type service struct {
	backend            transaction.Backend
	transactionService transaction.Service
	address            common.Address

	lockup *common.Address
}

func New(backend transaction.Backend, transactionService transaction.Service, address common.Address) mine.MineContract {
	return &service{
		backend:            backend,
		transactionService: transactionService,
		address:            address,
	}
}

func (s *service) MinersReceived(ctx context.Context, node common.Hash) (common.Address, error) {
	callData, err := mineABI.Pack("miners", node)
	if err != nil {
		return common.Address{}, err
	}

	output, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return common.Address{}, err
	}

	results, err := mineABI.Unpack("miners", output)
	if err != nil {
		return common.Address{}, err
	}
	if len(results) != 6 {
		return common.Address{}, errDecodeABI
	}
	return results[5].(common.Address), nil
}

func (s *service) MinersWithdraw(ctx context.Context, node common.Hash) (*big.Int, error) {
	callData, err := mineABI.Pack("miners", node)
	if err != nil {
		return nil, err
	}

	output, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return nil, err
	}

	results, err := mineABI.Unpack("miners", output)
	if err != nil {
		return nil, err
	}
	if len(results) != 6 {
		return nil, errDecodeABI
	}

	balance, ok := abi.ConvertType(results[4], new(big.Int)).(*big.Int)
	if !ok || balance == nil {
		return nil, errDecodeABI
	}
	return balance, nil
}

func (s *service) Reward(ctx context.Context, node common.Hash) (*big.Int, error) {
	callData, err := mineABI.Pack(`reward`, node)
	if err != nil {
		return nil, err
	}

	output, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return nil, err
	}

	results, err := mineABI.Unpack("reward", output)
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

func (s *service) Token(ctx context.Context) (common.Address, error) {
	callData, err := mineABI.Pack(`token`)
	if err != nil {
		return common.Address{}, err
	}
	data, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return common.Address{}, err
	}

	return common.BytesToAddress(data), nil
}

func (s *service) Lockup(ctx context.Context) (common.Address, error) {
	callData, err := mineABI.Pack(`lockup`)
	if err != nil {
		return common.Address{}, err
	}
	data, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return common.Address{}, err
	}

	return common.BytesToAddress(data), nil
}

func (s *service) ExpireOf(ctx context.Context, node common.Hash) (*big.Int, error) {
	callData, err := mineABI.Pack(`expireOf`, node)
	if err != nil {
		return nil, err
	}

	output, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return nil, err
	}

	results, err := mineABI.Unpack("expireOf", output)
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

func (s *service) FreezeOf(ctx context.Context, node common.Hash) (bool, error) {
	callData, err := mineABI.Pack(`freezeOf`, node)
	if err != nil {
		return false, err
	}

	output, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return false, err
	}

	results, err := mineABI.Unpack("freezeOf", output)
	if err != nil {
		return false, err
	}
	if len(results) != 1 {
		return false, errDecodeABI
	}

	return results[0].(bool), nil
}

func (s *service) DepositOf(ctx context.Context, node common.Hash) (*big.Int, error) {
	if s.lockup == nil {
		lockup, err := s.Lockup(ctx)
		if err != nil {
			return nil, err
		}
		s.lockup = &lockup
	}

	callData, err := lockupABI.Pack(`depositOf`, node)
	if err != nil {
		return nil, err
	}

	output, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   s.lockup,
		Data: callData,
	})
	if err != nil {
		return nil, err
	}

	results, err := lockupABI.Unpack("depositOf", output)
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

func (s *service) Unfreeze(ctx context.Context, node common.Hash) (common.Hash, error) {
	callData, err := mineABI.Pack(`unfreeze`, node)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &s.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		Value:       big.NewInt(0),
		Description: "unfreeze ",
	}

	txHash, err := s.transactionService.Send(ctx, request)
	if err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}

func (s *service) Withdraw(ctx context.Context, node common.Hash) (common.Hash, error) {
	callData, err := mineABI.Pack(`withdraw`, node)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &s.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		GasLimit:    9000000,
		Value:       big.NewInt(0),
		Description: "withdraw ",
	}

	txHash, err := s.transactionService.Send(ctx, request)
	if err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}

func (s *service) ValidateTrusts(ctx context.Context) (*big.Int, error) {
	callData, err := mineABI.Pack(`validateTrusts`)
	if err != nil {
		return nil, err
	}

	output, err := s.transactionService.Call(ctx, &transaction.TxRequest{
		To:   &s.address,
		Data: callData,
	})
	if err != nil {
		return nil, err
	}

	results, err := mineABI.Unpack("validateTrusts", output)
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, errDecodeABI
	}

	ret, ok := abi.ConvertType(results[0], new(big.Int)).(*big.Int)
	if !ok || ret == nil {
		return nil, errDecodeABI
	}
	return ret, nil
}

func (s *service) Deposit(ctx context.Context, node common.Hash, cate, price *big.Int, deadline *big.Int, signatures []byte) (common.Hash, error) {
	callData, err := mineABI.Pack("deposit", node, cate, price, deadline, signatures)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &s.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		GasLimit:    300000,
		Value:       big.NewInt(0),
		Description: "deposit",
	}

	return s.transactionService.Send(ctx, request)
}

func (s *service) WaitForDeposit(ctx context.Context, txHash common.Hash) error {
	receipt, err := s.transactionService.WaitForReceipt(ctx, txHash)
	if err != nil {
		return err
	}
	if receipt.Status != 1 {
		return fmt.Errorf("mine deposit failed")
	}
	return nil
}

func (s *service) CashDeposit(ctx context.Context, node common.Hash) (common.Hash, error) {
	callData, err := mineABI.Pack("cashDeposit", node)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &s.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		GasLimit:    200000,
		Value:       big.NewInt(0),
		Description: "cashDeposit",
	}

	return s.transactionService.Send(ctx, request)
}

func (s *service) Active(ctx context.Context, node common.Hash, cate, deadline *big.Int, signatures []byte) (common.Hash, error) {
	callData, err := mineABI.Pack("active", node, cate, deadline, signatures)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &s.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		GasLimit:    200000,
		Value:       big.NewInt(0),
		Description: "active",
	}

	return s.transactionService.Send(ctx, request)
}

func (s *service) WaitForActive(ctx context.Context, hash common.Hash) error {
	receipt, err := s.transactionService.WaitForReceipt(ctx, hash)
	if err != nil {
		return err
	}
	if receipt.Status != 1 {
		return fmt.Errorf("mine active failed")
	}
	return nil
}

func (s *service) Inactives(ctx context.Context, nodes []common.Hash, deadline *big.Int, signatures []byte) (common.Hash, error) {
	callData, err := mineABI.Pack("inactives", nodes, deadline, signatures)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:       &s.address,
		Data:     callData,
		GasPrice: sctx.GetGasPrice(ctx),
		// GasLimit:    200000,
		Value:       big.NewInt(0),
		Description: "inactives",
	}

	return s.transactionService.Send(ctx, request)
}

func (s *service) Dishonesty(ctx context.Context, node common.Hash, deadline *big.Int, signatures []byte) (common.Hash, error) {
	callData, err := mineABI.Pack("dishonesty", node)
	if err != nil {
		return common.Hash{}, err
	}

	request := &transaction.TxRequest{
		To:          &s.address,
		Data:        callData,
		GasPrice:    sctx.GetGasPrice(ctx),
		GasLimit:    200000,
		Value:       big.NewInt(0),
		Description: "dishonesty",
	}

	return s.transactionService.Send(ctx, request)
}

func (svc *service) CheckDeposit(ctx context.Context, node common.Hash) (bool, error) {
	callData, err := mineABI.Pack(`miners`, node)
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

	results, err := mineABI.Unpack("miners", output)
	if err != nil {
		return false, err
	}
	if len(results) != 6 {
		return false, errDecodeABI
	}
	return results[2].(bool), nil
}

func (svc *service) IsWorking(ctx context.Context, node common.Hash) (bool, error) {
	callData, err := mineABI.Pack(`miners`, node)
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

	results, err := mineABI.Unpack("miners", output)
	if err != nil {
		return false, err
	}
	if len(results) != 6 {
		return false, errDecodeABI
	}
	return results[0].(bool), nil
}
