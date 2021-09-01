// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nodestore

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/logging"
	"github.com/ethsana/sana/pkg/mine"
	"github.com/ethsana/sana/pkg/storage"
)

const (
	nodeKeyPrefix = "nodestore_node_"
	chainStateKey = "nodestore_chainstate"
)

type store struct {
	store storage.StateStorer // State store backend to persist batches.
	cs    *mine.ChainState    // the chain state

	logger logging.Logger
}

func New(st storage.StateStorer, startBlock uint64, logger logging.Logger) (mine.Storer, error) {
	cs := &mine.ChainState{}
	err := st.Get(chainStateKey, cs)

	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
		cs = &mine.ChainState{Block: startBlock}
	}
	return &store{st, cs, logger}, nil
}

func (s *store) Get(id common.Hash) (*mine.Node, error) {
	n := &mine.Node{}
	err := s.store.Get(nodeKey(id), n)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id.String(), err)
	}

	return n, nil
}

func (s *store) Put(n *mine.Node) error {
	return s.store.Put(nodeKey(n.Node), n)
}

func (s *store) Miners() ([]*mine.Node, error) {
	nodes := make([]*mine.Node, 0, 1024)
	err := s.store.Iterate(nodeKeyPrefix, func(key, value []byte) (stop bool, err error) {
		if strings.HasPrefix(string(key), nodeKeyPrefix) {
			var node mine.Node
			err = node.UnmarshalBinary(value)
			if err != nil {
				return true, err
			}

			nodes = append(nodes, &node)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (s *store) PutChainState(cs *mine.ChainState) error {
	s.cs = cs
	return s.store.Put(chainStateKey, cs)
}

func (s *store) GetChainState() *mine.ChainState {
	return s.cs
}

func (s *store) Reset(startBlock uint64) error {
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
	s.cs = &mine.ChainState{Block: startBlock}
	return nil
}

func nodeKey(hash common.Hash) string {
	return nodeKeyPrefix + string(hash[:])
}
