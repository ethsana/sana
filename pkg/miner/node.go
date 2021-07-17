// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package miner

// Batch represents a postage batch, a payment on the blockchain.
type Node struct {
	Node       []byte // node id
	Trust      bool   // trust node
	Chequebook []byte // chequebook
}

// MarshalBinary implements BinaryMarshaller. It will attempt to serialize the
// postage batch to a byte slice.
// serialised as ID(32)|big endian value(32)|start block(8)|owner addr(20)|BucketDepth(1)|depth(1)|immutable(1)
func (n *Node) MarshalBinary() ([]byte, error) {
	out := make([]byte, 65)
	copy(out, n.Node)
	if n.Trust {
		out[32] = 1
	}
	copy(out[33:], n.Chequebook)
	return out, nil
}

// UnmarshalBinary implements BinaryUnmarshaller. It will attempt deserialize
// the given byte slice into the batch.
func (n *Node) UnmarshalBinary(buf []byte) error {
	n.Node = buf[:32]
	n.Trust = buf[32] > 0
	n.Chequebook = buf[33:]
	return nil
}
