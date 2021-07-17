// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package miner

// ChainState contains data the batch service reads from the chain.
type ChainState struct {
	Block uint64 // The block number of the last postage event.
}
