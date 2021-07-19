// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nodeservice

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/logging"
	"github.com/ethsana/sana/pkg/mine"
	"github.com/ethsana/sana/pkg/postage"
	"github.com/ethsana/sana/pkg/storage"
	"github.com/ethsana/sana/pkg/swarm"
)

const (
	dirtyDBKey = "nodeservice_dirty_db"
)

type service struct {
	stateStore storage.StateStorer
	storer     mine.Storer
	logger     logging.Logger
	listener   postage.Listener

	nodes    map[common.Hash]*mine.Node
	nodesMtx sync.RWMutex

	synced         uint32
	startBlock     uint64
	rollcallSig    []chan uint64
	rollcallSigMtx sync.Mutex
}

// New will create a new nodeService.
func New(
	stateStore storage.StateStorer,
	storer mine.Storer,
	logger logging.Logger,
	listener postage.Listener,
	startBlock uint64,
) (mine.NodeService, error) {
	nodes, err := storer.Miners()
	if err != nil {
		return nil, err
	}
	dict := make(map[common.Hash]*mine.Node, len(nodes))
	for _, n := range nodes {
		dict[n.Node] = n
	}

	s := service{
		stateStore: stateStore,
		storer:     storer,
		logger:     logger,
		listener:   listener,
		nodes:      dict,
		startBlock: startBlock,
	}
	return &s, nil
}

func (s *service) Miner(node []byte, deposit, active bool, txHash []byte, blockNumber uint64) error {
	s.nodesMtx.Lock()
	defer s.nodesMtx.Unlock()
	id := common.BytesToHash(node)

	n, ok := s.nodes[id]
	if !ok {
		n = &mine.Node{Node: id, Trust: false, Active: active, Deposit: deposit, LastBlock: blockNumber}
		s.nodes[n.Node] = n
	}
	if active && n.LastBlock < blockNumber {
		n.LastBlock = blockNumber
	}
	n.Active = active
	n.Deposit = deposit

	err := s.storer.Put(n)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}

	s.logger.Debugf("mine service: mine %s active: %v deposit: %v height: %v, tx %x", n.Node.String(), n.Active, n.Deposit, blockNumber, txHash)
	return nil
}

func (s *service) Trust(node []byte, trust bool, txHash []byte) error {
	s.nodesMtx.Lock()
	defer s.nodesMtx.Unlock()
	id := common.BytesToHash(node)

	n, ok := s.nodes[id]
	if !ok {
		return fmt.Errorf("get: not found")
	}

	n.Trust = trust
	err := s.storer.Put(n)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}

	s.logger.Debugf("mine service: mine %s trust to %v, tx %x", n.Node.String(), trust, txHash)
	return nil
}

func (s *service) UpdateBlockNumber(blockNumber uint64) error {
	cs := s.storer.GetChainState()
	if blockNumber == cs.Block {
		return nil
	}

	cs.Block = blockNumber
	if err := s.storer.PutChainState(cs); err != nil {
		return fmt.Errorf("put chain state: %w", err)
	}

	if atomic.LoadUint32(&s.synced) == 1 {
		if (blockNumber-s.startBlock)%60 == 0 {
			s.rollcallSigMtx.Lock()
			for _, v := range s.rollcallSig {
				v <- blockNumber
			}
			s.rollcallSigMtx.Unlock()
		}
	}

	s.logger.Debugf("mine service: updated block height to %d", blockNumber)
	return nil
}

func (s *service) TransactionStart() error {
	return s.stateStore.Put(dirtyDBKey, true)
}

func (s *service) TransactionEnd() error {
	return s.stateStore.Delete(dirtyDBKey)
}

func (s *service) Start(startBlock uint64) (<-chan struct{}, error) {
	dirty := false
	err := s.stateStore.Get(dirtyDBKey, &dirty)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if dirty {
		s.logger.Warning("mine service: dirty shutdown detected, resetting batch store")
		if err := s.storer.Reset(); err != nil {
			return nil, err
		}
		if err := s.stateStore.Delete(dirtyDBKey); err != nil {
			return nil, err
		}
		s.logger.Warning("mine service: node store reset. your node will now resync chain data")
	}

	cs := s.storer.GetChainState()
	if cs.Block > startBlock {
		startBlock = cs.Block
	}

	syncedChan := s.listener.Listen(startBlock+1, s)
	go func() {
		<-syncedChan
		<-time.After(time.Second * 2)
		atomic.StoreUint32(&s.synced, 1)
	}()
	return syncedChan, nil
}

func (s *service) TrustAddress(filter func(swarm.Address) bool) ([]swarm.Address, error) {
	s.nodesMtx.RLock()
	defer s.nodesMtx.RUnlock()
	addrs := make([]swarm.Address, 0, len(s.nodes))
	for _, n := range s.nodes {
		if n.Trust {
			addr := swarm.NewAddress(n.Node[:])
			if filter == nil {
				addrs = append(addrs, addr)
				continue
			}
			if filter(addr) {
				addrs = append(addrs, addr)
			}
		}
	}
	return addrs, nil
}

func (s *service) TrustOf(node swarm.Address) bool {
	s.nodesMtx.RLock()
	defer s.nodesMtx.RUnlock()
	if v, ok := s.nodes[common.BytesToHash(node.Bytes())]; ok {
		return v.Trust
	}
	return false
}

func (s *service) UpdateNodeLastBlock(node swarm.Address, blockNumber uint64) error {
	s.nodesMtx.Lock()
	defer s.nodesMtx.Unlock()
	id := common.BytesToHash(node.Bytes())

	n, ok := s.nodes[id]
	if !ok {
		return fmt.Errorf("%v not found", id.String())
	}

	n.LastBlock = blockNumber
	err := s.storer.Put(n)
	if err != nil {
		return fmt.Errorf("put: %w", err)
	}

	s.logger.Debugf("mine service: mine %s lastblock %v", n.Node.String(), n.LastBlock)
	return nil
}

func (s *service) ExpireMiners() ([]swarm.Address, error) {
	s.nodesMtx.Lock()
	defer s.nodesMtx.Unlock()

	state := s.storer.GetChainState()
	list := make([]swarm.Address, 0, len(s.nodes))
	for _, n := range s.nodes {
		if n.Active && n.LastBlock < state.Block-5 {
			list = append(list, swarm.NewAddress(n.Node[:]))
		}
	}
	return list, nil
}

func (s *service) SubscribeRollCall() (c <-chan uint64, unsubscribe func()) {
	channel := make(chan uint64, 1)
	var closeOnce sync.Once

	s.rollcallSigMtx.Lock()
	defer s.rollcallSigMtx.Unlock()

	s.rollcallSig = append(s.rollcallSig, channel)

	unsubscribe = func() {
		s.rollcallSigMtx.Lock()
		defer s.rollcallSigMtx.Unlock()

		for i, c := range s.rollcallSig {
			if c == channel {
				s.rollcallSig = append(s.rollcallSig[:i], s.rollcallSig[i+1:]...)
				break
			}
		}

		closeOnce.Do(func() { close(channel) })
	}
	return channel, unsubscribe
}
