// Copyright 2020 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mine

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/swarm"
	"golang.org/x/net/context"
)

const (
	OPSIGN int32 = iota
)

// EventUpdater interface definitions reflect the updates triggered by events
// emitted by the postage contract on the blockchain.
type EventUpdater interface {
	Miner(node []byte, deposit, active bool, txHash []byte, blockNumber uint64) error
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
	TrustOf(node swarm.Address) bool
	UpdateNodeLastBlock(node swarm.Address, blockNumber uint64) error
	TrustAddress(filter func(swarm.Address) bool) ([]swarm.Address, error)
	ExpireMiners() ([]swarm.Address, error)
	SubscribeRollCall() (<-chan uint64, func())
}

// Storer represents the persistence layer for batches on the current (highest
// available) block.
type Storer interface {
	Get(id common.Hash) (*Node, error)
	Put(*Node) error
	Miners() ([]*Node, error)
	PutChainState(*ChainState) error
	GetChainState() *ChainState
	// GetReserveState() *ReserveState
	// SetRadiusSetter(RadiusSetter)
	// Unreserve(UnreserveIteratorFn) error

	Reset() error
}

type MineContract interface {
	IsWorking(ctx context.Context, node common.Hash) (bool, error)
	Lockup(ctx context.Context) (common.Address, error)
	MinersReceived(ctx context.Context, node common.Hash) (common.Address, error)
	MinersWithdraw(ctx context.Context, node common.Hash) (*big.Int, error)
	ExpireOf(ctx context.Context, node common.Hash) (*big.Int, error)
	Reward(ctx context.Context, node common.Hash) (*big.Int, error)
	CheckDeposit(ctx context.Context, node common.Hash) (bool, error)
	Withdraw(ctx context.Context, node common.Hash, deadline *big.Int, sign []byte) (common.Hash, error)
	ValidateTrusts(ctx context.Context) (*big.Int, error)
	Deposit(ctx context.Context, node common.Hash) (common.Hash, error)
	WaitForDeposit(ctx context.Context, hash common.Hash) error
	CashDeposit(ctx context.Context, node common.Hash) (common.Hash, error)
	Active(ctx context.Context, node common.Hash, deadline *big.Int, signatures []byte) (common.Hash, error)
	WaitForActive(ctx context.Context, hash common.Hash) error
	Inaction(ctx context.Context, node common.Hash, deadline *big.Int, signatures []byte) (common.Hash, error)
	Dishonesty(ctx context.Context, node common.Hash, deadline *big.Int, signatures []byte) (common.Hash, error)
}

type Trust interface {
	TrustsSignature(ctx context.Context, op int32, expire int64, data []byte, peer ...swarm.Address) ([]byte, error)
	PushSignatures(ctx context.Context, id, op int32, expire int64, data []byte, target swarm.Address) error
	PushTrustSign(ctx context.Context, op int32, expire int64, data []byte, target swarm.Address) error
	PushRollCall(ctx context.Context, op int32, expire int64, data []byte, skips ...swarm.Address) error
}
