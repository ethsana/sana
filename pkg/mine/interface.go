// Copyright 2020 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mine

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethersphere/bee/pkg/swarm"
	"golang.org/x/net/context"
)

// EventUpdater interface definitions reflect the updates triggered by events
// emitted by the postage contract on the blockchain.
type EventUpdater interface {
	Miner(node, chequebook, txHash []byte) error
	Trust(node []byte, trust bool, txHash []byte) error

	// Create(id []byte, owner []byte, normalisedBalance *big.Int, depth, bucketDepth uint8, immutable bool, txHash []byte) error
	// TopUp(id []byte, normalisedBalance *big.Int, txHash []byte) error
	// UpdateDepth(id []byte, depth uint8, normalisedBalance *big.Int, txHash []byte) error
	// UpdatePrice(price *big.Int, txHash []byte) error
	UpdateBlockNumber(blockNumber uint64) error
	// Start(startBlock uint64) (<-chan struct{}, error)

	TransactionStart() error
	TransactionEnd() error
}

type NodeService interface {
	Start(startBlock uint64) (<-chan struct{}, error)
	TrustAddress() ([]swarm.Address, error)
}

// Storer represents the persistence layer for batches on the current (highest
// available) block.
type Storer interface {
	Get(node []byte) (*Node, error)
	Put(*Node) error
	PutChainState(*ChainState) error
	GetChainState() *ChainState
	// GetReserveState() *ReserveState
	// SetRadiusSetter(RadiusSetter)
	// Unreserve(UnreserveIteratorFn) error

	Reset() error
}

type MinerContract interface {
	IsMiner(ctx context.Context, node common.Hash) (bool, error)
	Regist(ctx context.Context, node common.Hash, chequebook common.Address) (common.Hash, error)
	WaitRegist(ctx context.Context, hash common.Hash) (uint64, error)
	Reward(ctx context.Context, node common.Hash) (*big.Int, error)
	Withdraw(ctx context.Context, node common.Hash, deadline *big.Int, sign []byte) (common.Hash, error)
}

type RollCall interface {
	Certificate(ctx context.Context, peer swarm.Address, data []byte) ([]byte, error)
}
