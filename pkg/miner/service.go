// Copyright 2021 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package miner

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethersphere/bee/pkg/crypto"
	"github.com/ethersphere/bee/pkg/logging"
	"github.com/ethersphere/bee/pkg/sctx"
	"github.com/ethersphere/bee/pkg/swarm"
)

const (
	minerPrefix = "miner"
)

var (
	// ErrNotFound is the error returned when issuer with given batch ID does not exist.
	ErrNotFound = errors.New("not found")
	// ErrNotUsable is the error returned when issuer with given batch ID is not usable.
	ErrNotUsable = errors.New("not usable")
)

// Service is the miner service interface.
type Service interface {
	Start(startBlock uint64) (<-chan struct{}, error)
	SetRollCall(rollcall RollCall)
	NotifyCertificate(peer swarm.Address, data []byte) ([]byte, error)

	Pending(ctx context.Context) (*big.Int, error)
	Withdraw(ctx context.Context) (common.Hash, error)
}

// service handles postage batches
// stores the active batches.
type service struct {
	base     swarm.Address
	signer   crypto.Signer
	nodes    NodeService
	contract MinerContract
	rollcall RollCall
	logger   logging.Logger
}

// NewService constructs a new Service.
func NewService(base swarm.Address, chequebook common.Address, contract MinerContract, nodes NodeService, signer crypto.Signer, logger logging.Logger, deployGasPrice string) (Service, error) {
	ctx := context.Background()
	if deployGasPrice != "" {
		gasPrice, ok := new(big.Int).SetString(deployGasPrice, 10)
		if !ok {
			return nil, fmt.Errorf("deploy gas price \"%s\" cannot be parsed", deployGasPrice)
		}
		ctx = sctx.SetGasPrice(ctx, gasPrice)
	}

	node := common.BytesToHash(base.Bytes())
	ok, err := contract.IsMiner(ctx, node)
	if err != nil {
		return nil, err
	}
	if !ok {
		hash, err := contract.Regist(ctx, node, chequebook)
		if err != nil {
			return nil, err
		}
		status, err := contract.WaitRegist(ctx, hash)
		if err != nil {
			return nil, err
		}
		if status != 1 {
			return nil, fmt.Errorf(`miner not regist success`)
		}
	}

	return &service{
		base:     base,
		signer:   signer,
		contract: contract,
		nodes:    nodes,
		logger:   logger,
	}, nil
}

func (s *service) SetRollCall(rollCall RollCall) {
	s.rollcall = rollCall
}

func (s *service) NotifyCertificate(peer swarm.Address, data []byte) ([]byte, error) {
	// todo check peer is active
	deadline := big.NewInt(time.Now().Add(time.Minute).Unix())
	deadbyts := deadline.Bytes()
	byts := make([]byte, 64)
	copy(byts[:32], peer.Bytes())
	copy(byts[32:], common.BigToHash(deadline).Bytes())
	byts, err := crypto.LegacyKeccak256(byts)
	if err != nil {
		return nil, err
	}
	byts, err = s.signer.Sign(byts)
	if err != nil {
		return nil, err
	}
	return append(byts, deadbyts...), nil
}

func (s *service) Pending(ctx context.Context) (*big.Int, error) {
	node := common.BytesToHash(s.base.Bytes())
	return s.contract.Reward(ctx, node)
}

func (s *service) Withdraw(ctx context.Context) (common.Hash, error) {
	node := common.BytesToHash(s.base.Bytes())
	addrs, err := s.nodes.TrustAddress()
	if err != nil {
		return common.Hash{}, err
	}

	trust := addrs[len(addrs)-1]
	if trust.Equal(s.base) {
		byts, err := s.NotifyCertificate(s.base, nil)
		if err != nil {
			return common.Hash{}, fmt.Errorf(`trust node sign fail`)
		}
		deadline := new(big.Int).SetBytes(byts[65:])
		return s.contract.Withdraw(ctx, node, deadline, byts[:65])
	} else {
		byts, err := s.rollcall.Certificate(ctx, trust, s.base.Bytes())
		if err != nil {
			return common.Hash{}, fmt.Errorf(`trust node sign fail`)
		}
		deadline := new(big.Int).SetBytes(byts[65:])
		return s.contract.Withdraw(ctx, node, deadline, byts[:65])
	}
}

func (s *service) Start(startBlock uint64) (<-chan struct{}, error) {
	return s.nodes.Start(startBlock)
}

// Close saves all the active stamp issuers to statestore.
func (s *service) Close() error {

	return nil
}
