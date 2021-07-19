// Copyright 2021 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feeds

import (
	"context"
	"encoding/binary"

	"github.com/ethsana/sana/pkg/cac"
	"github.com/ethsana/sana/pkg/crypto"
	"github.com/ethsana/sana/pkg/soc"
	"github.com/ethsana/sana/pkg/storage"
	"github.com/ethsana/sana/pkg/swarm"
)

// Updater is the generic interface f
type Updater interface {
	Update(ctx context.Context, at int64, payload []byte) error
	Feed() *Feed
}

// Putter encapsulates a chunk store putter and a Feed to store feed updates
type Putter struct {
	putter storage.Putter
	signer crypto.Signer
	*Feed
}

// NewPutter constructs a feed Putter
func NewPutter(putter storage.Putter, signer crypto.Signer, topic []byte) (*Putter, error) {
	owner, err := signer.EthereumAddress()
	if err != nil {
		return nil, err
	}
	feed := New(topic, owner)
	return &Putter{putter, signer, feed}, nil
}

// Put pushes an update to the feed through the chunk stores
func (u *Putter) Put(ctx context.Context, i Index, at int64, payload []byte) error {
	id, err := u.Feed.Update(i).Id()
	if err != nil {
		return err
	}
	cac, err := toChunk(uint64(at), payload)
	if err != nil {
		return err
	}
	s := soc.New(id, cac)
	ch, err := s.Sign(u.signer)
	if err != nil {
		return err
	}
	_, err = u.putter.Put(ctx, storage.ModePutUpload, ch)
	return err
}

func toChunk(at uint64, payload []byte) (swarm.Chunk, error) {
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, at)
	return cac.New(append(ts, payload...))
}
