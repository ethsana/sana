// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package postage

import (
	"math/big"

	"github.com/ethsana/sana/pkg/syncer"
)

// EventUpdater interface definitions reflect the updates triggered by events
// emitted by the postage contract on the blockchain.
type EventUpdater interface {
	Sync() *syncer.Sync
	Create(id []byte, owner []byte, normalisedBalance *big.Int, depth, bucketDepth uint8, immutable bool, txHash []byte) error
	TopUp(id []byte, normalisedBalance *big.Int, txHash []byte) error
	UpdateDepth(id []byte, depth uint8, normalisedBalance *big.Int, txHash []byte) error
	UpdatePrice(price *big.Int, txHash []byte) error
	UpdateBlockNumber(blockNumber uint64) error
	// Start(startBlock uint64) (<-chan struct{}, error)

	TransactionStart() error
	TransactionEnd() error
}

type UnreserveIteratorFn func(id []byte, radius uint8) (bool, error)

// Storer represents the persistence layer for batches on the current (highest
// available) block.
type Storer interface {
	Get(id []byte) (*Batch, error)
	Put(*Batch, *big.Int, uint8) error
	GetChainState() *ChainState
	PutChainState(*ChainState) error
	GetReserveState() *ReserveState
	SetRadiusSetter(RadiusSetter)
	Unreserve(UnreserveIteratorFn) error
	Exists(id []byte) (bool, error)

	Reset(startBlock uint64) error
}

type RadiusSetter interface {
	SetRadius(uint8)
}

type BatchCreationListener interface {
	Handle(*Batch)
}
