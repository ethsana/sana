// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mine_test

import (
	"fmt"
	"testing"

	"github.com/ethersphere/bee/pkg/miner"
)

// TestBatchMarshalling tests the idempotence  of binary marshal/unmarshal for a
// Batch.
func TestNodeMarshalling(t *testing.T) {
	node := &miner.Node{[]byte(`id`), true, []byte(`chequebook`)}
	byts, err := node.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	node1 := new(miner.Node)
	err = node1.UnmarshalBinary(byts)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(node1.Node), node1.Trust, string(node1.Chequebook))
}
