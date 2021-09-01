// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package batchservice

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethersphere/go-storage-incentives-abi/postageabi"
	"github.com/ethsana/sana/pkg/logging"
	"github.com/ethsana/sana/pkg/postage"
	"github.com/ethsana/sana/pkg/storage"
	"github.com/ethsana/sana/pkg/syncer"
	"github.com/ethsana/sana/pkg/transaction"
	"golang.org/x/crypto/sha3"
)

const (
	dirtyDBKey    = "batchservice_dirty_db"
	checksumDBKey = "batchservice_checksum"
)

var (
	postageStampABI = transaction.ParseABIUnchecked(postageabi.PostageStampABIv0_3_0)
	// batchCreatedTopic is the postage contract's batch created event topic
	batchCreatedTopic = postageStampABI.Events["BatchCreated"].ID
	// batchTopupTopic is the postage contract's batch topup event topic
	batchTopupTopic = postageStampABI.Events["BatchTopUp"].ID
	// batchDepthIncreaseTopic is the postage contract's batch dilution event topic
	batchDepthIncreaseTopic = postageStampABI.Events["BatchDepthIncrease"].ID
	// priceUpdateTopic is the postage contract's price update event topic
	priceUpdateTopic = postageStampABI.Events["PriceUpdate"].ID
)

type batchService struct {
	stateStore    storage.StateStorer
	storer        postage.Storer
	logger        logging.Logger
	address       common.Address
	owner         []byte
	batchListener postage.BatchCreationListener

	checksum hash.Hash // checksum hasher
}

type Interface interface {
	postage.EventUpdater
	ProcessEvent(e types.Log) error
}

// New will create a new BatchService.
func New(
	stateStore storage.StateStorer,
	storer postage.Storer,
	logger logging.Logger,
	address common.Address,
	owner []byte,
	startBlock uint64,
	batchListener postage.BatchCreationListener,
	checksumFunc func() hash.Hash,
) (Interface, error) {
	if checksumFunc == nil {
		checksumFunc = sha3.New256
	}
	var (
		b   string
		sum = checksumFunc()
	)

	if err := stateStore.Get(checksumDBKey, &b); err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
	} else {
		s, err := hex.DecodeString(b)
		if err != nil {
			return nil, err
		}
		n, err := sum.Write(s)
		if err != nil {
			return nil, err
		}
		if n != len(s) {
			return nil, errors.New("batchstore checksum init")
		}
	}

	dirty := false
	err := stateStore.Get(dirtyDBKey, &dirty)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if dirty {
		logger.Warning("batch service: dirty shutdown detected, resetting batch store")
		if err := storer.Reset(startBlock); err != nil {
			return nil, err
		}
		if err := stateStore.Delete(dirtyDBKey); err != nil {
			return nil, err
		}
		logger.Warning("batch service: batch store reset. your node will now resync chain data")
	}
	return &batchService{stateStore, storer, logger, address, owner, batchListener, sum}, nil
}

func (svc *batchService) Sync() *syncer.Sync {
	return &syncer.Sync{
		From:       svc.storer.GetChainState().Block + 1,
		FilterLogs: svc.filterLogs,
		Updater:    svc,
	}
}

func (svc *batchService) filterLogs(from, to *big.Int) ethereum.FilterQuery {
	return ethereum.FilterQuery{
		FromBlock: from,
		ToBlock:   to,
		Addresses: []common.Address{svc.address},
		Topics: [][]common.Hash{{
			batchCreatedTopic,
			batchTopupTopic,
			batchDepthIncreaseTopic,
			priceUpdateTopic,
		}},
	}
}

type batchCreatedEvent struct {
	BatchId           [32]byte
	TotalAmount       *big.Int
	NormalisedBalance *big.Int
	Owner             common.Address
	Depth             uint8
	BucketDepth       uint8
	ImmutableFlag     bool
}

type batchTopUpEvent struct {
	BatchId           [32]byte
	TopupAmount       *big.Int
	NormalisedBalance *big.Int
}

type batchDepthIncreaseEvent struct {
	BatchId           [32]byte
	NewDepth          uint8
	NormalisedBalance *big.Int
}

type priceUpdateEvent struct {
	Price *big.Int
}

func (svc *batchService) ProcessEvent(e types.Log) error {
	// defer l.metrics.EventsProcessed.Inc()
	switch e.Topics[0] {
	case batchCreatedTopic:
		c := &batchCreatedEvent{}
		err := transaction.ParseEvent(&postageStampABI, "BatchCreated", c, e)
		if err != nil {
			return err
		}
		// l.metrics.CreatedCounter.Inc()
		return svc.Create(
			c.BatchId[:],
			c.Owner.Bytes(),
			c.NormalisedBalance,
			c.Depth,
			c.BucketDepth,
			c.ImmutableFlag,
			e.TxHash.Bytes(),
		)

	case batchTopupTopic:
		c := &batchTopUpEvent{}
		err := transaction.ParseEvent(&postageStampABI, "BatchTopUp", c, e)
		if err != nil {
			return err
		}
		// l.metrics.TopupCounter.Inc()
		return svc.TopUp(
			c.BatchId[:],
			c.NormalisedBalance,
			e.TxHash.Bytes(),
		)
	case batchDepthIncreaseTopic:
		c := &batchDepthIncreaseEvent{}
		err := transaction.ParseEvent(&postageStampABI, "BatchDepthIncrease", c, e)
		if err != nil {
			return err
		}
		// l.metrics.DepthCounter.Inc()
		return svc.UpdateDepth(
			c.BatchId[:],
			c.NewDepth,
			c.NormalisedBalance,
			e.TxHash.Bytes(),
		)
	case priceUpdateTopic:
		c := &priceUpdateEvent{}
		err := transaction.ParseEvent(&postageStampABI, "PriceUpdate", c, e)
		if err != nil {
			return err
		}
		// l.metrics.PriceCounter.Inc()
		return svc.UpdatePrice(
			c.Price,
			e.TxHash.Bytes(),
		)
	default:
		// l.metrics.EventErrors.Inc()
		return errors.New("unknown event")
	}
}

// Create will create a new batch with the given ID, owner value and depth and
// stores it in the BatchStore.
func (svc *batchService) Create(id, owner []byte, normalisedBalance *big.Int, depth, bucketDepth uint8, immutable bool, txHash []byte) error {
	b := &postage.Batch{
		ID:          id,
		Owner:       owner,
		Value:       big.NewInt(0),
		Start:       svc.storer.GetChainState().Block,
		Depth:       depth,
		BucketDepth: bucketDepth,
		Immutable:   immutable,
	}

	err := svc.storer.Put(b, normalisedBalance, depth)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}

	if bytes.Equal(svc.owner, owner) && svc.batchListener != nil {
		svc.batchListener.Handle(b)
	}
	cs, err := svc.updateChecksum(txHash)
	if err != nil {
		return fmt.Errorf("update checksum: %w", err)
	}

	svc.logger.Debugf("batch service: created batch id %s, tx %x, checksum %x", hex.EncodeToString(b.ID), txHash, cs)
	return nil
}

// TopUp implements the EventUpdater interface. It tops ups a batch with the
// given ID with the given amount.
func (svc *batchService) TopUp(id []byte, normalisedBalance *big.Int, txHash []byte) error {
	b, err := svc.storer.Get(id)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}

	err = svc.storer.Put(b, normalisedBalance, b.Depth)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}
	cs, err := svc.updateChecksum(txHash)
	if err != nil {
		return fmt.Errorf("update checksum: %w", err)
	}

	svc.logger.Debugf("batch service: topped up batch id %s from %v to %v, tx %x, checksum %x", hex.EncodeToString(b.ID), b.Value, normalisedBalance, txHash, cs)
	return nil
}

// UpdateDepth implements the EventUpdater inteface. It sets the new depth of a
// batch with the given ID.
func (svc *batchService) UpdateDepth(id []byte, depth uint8, normalisedBalance *big.Int, txHash []byte) error {
	b, err := svc.storer.Get(id)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	err = svc.storer.Put(b, normalisedBalance, depth)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}
	cs, err := svc.updateChecksum(txHash)
	if err != nil {
		return fmt.Errorf("update checksum: %w", err)
	}

	svc.logger.Debugf("batch service: updated depth of batch id %s from %d to %d, tx %x, checksum %x", hex.EncodeToString(b.ID), b.Depth, depth, txHash, cs)
	return nil
}

// UpdatePrice implements the EventUpdater interface. It sets the current
// price from the chain in the service chain state.
func (svc *batchService) UpdatePrice(price *big.Int, txHash []byte) error {
	cs := svc.storer.GetChainState()
	cs.CurrentPrice = price
	if err := svc.storer.PutChainState(cs); err != nil {
		return fmt.Errorf("put chain state: %w", err)
	}

	sum, err := svc.updateChecksum(txHash)
	if err != nil {
		return fmt.Errorf("update checksum: %w", err)
	}

	svc.logger.Debugf("batch service: updated chain price to %s, tx %x, checksum %x", price, txHash, sum)
	return nil
}

func (svc *batchService) UpdateBlockNumber(blockNumber uint64) error {
	cs := svc.storer.GetChainState()
	if blockNumber == cs.Block {
		return nil
	}
	diff := big.NewInt(0).SetUint64(blockNumber - cs.Block)

	cs.TotalAmount.Add(cs.TotalAmount, diff.Mul(diff, cs.CurrentPrice))
	cs.Block = blockNumber
	if err := svc.storer.PutChainState(cs); err != nil {
		return fmt.Errorf("put chain state: %w", err)
	}

	svc.logger.Debugf("batch service: updated block height to %d", blockNumber)
	return nil
}
func (svc *batchService) TransactionStart() error {
	return svc.stateStore.Put(dirtyDBKey, true)
}
func (svc *batchService) TransactionEnd() error {
	return svc.stateStore.Delete(dirtyDBKey)
}

func (svc *batchService) Start(startBlock uint64) (<-chan struct{}, error) {
	// dirty := false
	// err := svc.stateStore.Get(dirtyDBKey, &dirty)
	// if err != nil && !errors.Is(err, storage.ErrNotFound) {
	// 	return nil, err
	// }
	// if dirty {
	// 	svc.logger.Warning("batch service: dirty shutdown detected, resetting batch store")
	// 	if err := svc.storer.Reset(); err != nil {
	// 		return nil, err
	// 	}
	// 	if err := svc.stateStore.Delete(dirtyDBKey); err != nil {
	// 		return nil, err
	// 	}
	// 	svc.logger.Warning("batch service: batch store reset. your node will now resync chain data")
	// }

	// cs := svc.storer.GetChainState()
	// if cs.Block > startBlock {
	// 	startBlock = cs.Block
	// }
	// return svc.listener.Listen(startBlock+1, svc), nil
	return nil, nil
}

// updateChecksum updates the batchservice checksum once an event gets
// processed. It swaps the existing checksum which is in the hasher
// with the new checksum and persists it in the statestore.
func (svc *batchService) updateChecksum(txHash []byte) ([]byte, error) {
	n, err := svc.checksum.Write(txHash)
	if err != nil {
		return nil, err
	}
	if l := len(txHash); l != n {
		return nil, fmt.Errorf("update checksum wrote %d bytes but want %d bytes", n, l)
	}
	s := svc.checksum.Sum(nil)
	svc.checksum.Reset()
	n, err = svc.checksum.Write(s)
	if err != nil {
		return nil, err
	}
	if l := len(s); l != n {
		return nil, fmt.Errorf("swap checksum wrote %d bytes but want %d bytes", n, l)
	}

	b := hex.EncodeToString(s)

	return s, svc.stateStore.Put(checksumDBKey, b)
}
