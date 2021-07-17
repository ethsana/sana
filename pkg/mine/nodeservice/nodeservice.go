// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nodeservice

import (
	"encoding/hex"
	"errors"
	"fmt"
	"hash"

	"github.com/ethersphere/bee/pkg/logging"
	"github.com/ethersphere/bee/pkg/mine"
	"github.com/ethersphere/bee/pkg/postage"
	"github.com/ethersphere/bee/pkg/storage"
	"github.com/ethersphere/bee/pkg/swarm"
	"golang.org/x/crypto/sha3"
)

const (
	dirtyDBKey    = "nodeservice_dirty_db"
	checksumDBKey = "nodeservice_checksum"
)

type nodeService struct {
	stateStore storage.StateStorer
	storer     mine.Storer
	logger     logging.Logger
	listener   postage.Listener

	trusts   []swarm.Address
	checksum hash.Hash // checksum hasher
}

type Interface interface {
	mine.EventUpdater
	Start(startBlock uint64) (<-chan struct{}, error)

	TrustAddress() ([]swarm.Address, error)
}

// New will create a new nodeService.
func New(
	stateStore storage.StateStorer,
	storer mine.Storer,
	logger logging.Logger,
	listener postage.Listener,
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
			return nil, errors.New("nodestore checksum init")
		}
	}

	return &nodeService{stateStore, storer, logger, listener, nil, sum}, nil
}

func (svc *nodeService) Miner(node, chequebook []byte, txHash []byte) error {
	n := &mine.Node{
		Node:       node,
		Trust:      false,
		Chequebook: chequebook,
	}
	err := svc.storer.Put(n)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}
	cs, err := svc.updateChecksum(txHash)
	if err != nil {
		return fmt.Errorf("update checksum: %w", err)
	}

	svc.logger.Debugf("node service: received node address %s, tx %x, checksum %x", hex.EncodeToString(n.Node), txHash, cs)
	return nil
}

func (svc *nodeService) Trust(node []byte, trust bool, txHash []byte) error {
	n, err := svc.storer.Get(node)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}

	svc.trusts = append(svc.trusts, swarm.NewAddress(node))

	n.Trust = trust
	err = svc.storer.Put(n)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}
	cs, err := svc.updateChecksum(txHash)
	if err != nil {
		return fmt.Errorf("update checksum: %w", err)
	}

	svc.logger.Debugf("node service: update node %s trust to %v, tx %x, checksum %x", hex.EncodeToString(n.Node), trust, txHash, cs)
	return nil
}

func (svc *nodeService) UpdateBlockNumber(blockNumber uint64) error {
	cs := svc.storer.GetChainState()
	if blockNumber == cs.Block {
		return nil
	}

	cs.Block = blockNumber
	if err := svc.storer.PutChainState(cs); err != nil {
		return fmt.Errorf("put chain state: %w", err)
	}

	svc.logger.Debugf("node service: updated block height to %d", blockNumber)
	return nil
}
func (svc *nodeService) TransactionStart() error {
	return svc.stateStore.Put(dirtyDBKey, true)
}
func (svc *nodeService) TransactionEnd() error {
	return svc.stateStore.Delete(dirtyDBKey)
}

func (svc *nodeService) Start(startBlock uint64) (<-chan struct{}, error) {
	dirty := false
	err := svc.stateStore.Get(dirtyDBKey, &dirty)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if dirty {
		svc.logger.Warning("node service: dirty shutdown detected, resetting batch store")
		if err := svc.storer.Reset(); err != nil {
			return nil, err
		}
		if err := svc.stateStore.Delete(dirtyDBKey); err != nil {
			return nil, err
		}
		svc.logger.Warning("node service: batch store reset. your node will now resync chain data")
	}

	cs := svc.storer.GetChainState()
	if cs.Block > startBlock {
		startBlock = cs.Block
	}
	return svc.listener.Listen(startBlock+1, svc), nil
}

// updateChecksum updates the nodeservice checksum once an event gets
// processed. It swaps the existing checksum which is in the hasher
// with the new checksum and persists it in the statestore.
func (svc *nodeService) updateChecksum(txHash []byte) ([]byte, error) {
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

func (svc *nodeService) TrustAddress() ([]swarm.Address, error) {
	if len(svc.trusts) == 0 {
		return nil, fmt.Errorf("no found trust node")
	}
	return svc.trusts, nil
}
