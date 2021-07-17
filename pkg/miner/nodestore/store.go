// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nodestore

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/ethersphere/bee/pkg/logging"
	"github.com/ethersphere/bee/pkg/miner"
	"github.com/ethersphere/bee/pkg/storage"
)

const (
	nodeKeyPrefix  = "nodestore_batch_"
	valueKeyPrefix = "nodestore_value_"
	chainStateKey  = "nodestore_chainstate"
)

// store implements postage.Storer
type store struct {
	store storage.StateStorer // State store backend to persist batches.
	cs    *miner.ChainState   // the chain state

	logger logging.Logger
}

// New constructs a new postage batch store.
// It initialises both chain state and reserve state from the persistent state store
func New(st storage.StateStorer, logger logging.Logger) (miner.Storer, error) {
	cs := &miner.ChainState{}
	err := st.Get(chainStateKey, cs)

	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
		cs = &miner.ChainState{Block: 0}
	}
	return &store{st, cs, logger}, nil
}

// Get returns a batch from the batchstore with the given ID.
func (s *store) Get(id []byte) (*miner.Node, error) {
	n := &miner.Node{}
	err := s.store.Get(nodeKey(id), n)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", hex.EncodeToString(id), err)
	}

	return n, nil
}

// Put stores a given batch in the batchstore and requires new values of Value and Depth
func (s *store) Put(n *miner.Node) error {
	return s.store.Put(nodeKey(n.Node), n)
}

// PutChainState implements BatchStorer.
// It purges expired batches and unreserves underfunded ones before it
// stores the chain state in the batch store.
func (s *store) PutChainState(cs *miner.ChainState) error {
	s.cs = cs
	return s.store.Put(chainStateKey, cs)
}

// GetChainState implements BatchStorer. It returns the stored chain state from
// the batch store.
func (s *store) GetChainState() *miner.ChainState {
	return s.cs
}

func (s *store) Reset() error {
	prefix := "nodestore_"
	if err := s.store.Iterate(prefix, func(k, _ []byte) (bool, error) {
		if strings.HasPrefix(string(k), prefix) {
			if err := s.store.Delete(string(k)); err != nil {
				return false, err
			}
		}
		return false, nil
	}); err != nil {
		return err
	}
	s.cs = &miner.ChainState{Block: 0}
	return nil
}

// batchKey returns the index key for the batch ID used in the by-ID batch index.
func nodeKey(id []byte) string {
	return nodeKeyPrefix + string(id)
}
