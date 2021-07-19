// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mine

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
)

// Batch represents a postage batch, a payment on the blockchain.
type Node struct {
	Node      common.Hash // node id
	Trust     bool        // trust node
	Deposit   bool
	Active    bool
	LastBlock uint64
}

// MarshalBinary implements BinaryMarshaller. It will attempt to serialize the
// postage batch to a byte slice.
// serialised as ID(32)|big endian value(32)|start block(8)|owner addr(20)|BucketDepth(1)|depth(1)|immutable(1)
func (n *Node) MarshalBinary() ([]byte, error) {
	out := make([]byte, 43)
	copy(out, n.Node.Bytes())
	if n.Trust {
		out[32] = 1
	}
	if n.Active {
		out[33] = 1
	}
	if n.Deposit {
		out[34] = 1
	}
	binary.BigEndian.PutUint64(out[35:], n.LastBlock)
	return out, nil
}

// UnmarshalBinary implements BinaryUnmarshaller. It will attempt deserialize
// the given byte slice into the batch.
func (n *Node) UnmarshalBinary(buf []byte) error {
	n.Node = common.BytesToHash(buf[:32])
	n.Trust = buf[32] > 0
	n.Active = buf[33] > 0
	n.Deposit = buf[34] > 0
	n.LastBlock = binary.BigEndian.Uint64(buf[35:])
	return nil
}
