// Copyright 2020 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trust

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/logging"
	"github.com/ethsana/sana/pkg/mine/trust/pb"
	"github.com/ethsana/sana/pkg/p2p"
	"github.com/ethsana/sana/pkg/p2p/protobuf"
	"github.com/ethsana/sana/pkg/swarm"
	"github.com/ethsana/sana/pkg/topology"
)

const (
	protocolName       = "rollcall"
	protocolVersion    = "1.0.0"
	streamSign         = "sign"
	streamSignRet      = "signret"
	streamRollCall     = "rollcall"
	streamRollCallSign = "rollcallsign"
)

type MineObserver interface {
	NotifyTrustSignature(peer swarm.Address, expire int64, data []byte) error
	NotifyTrustRollCall(peer swarm.Address, expire int64, data []byte) error
	NotifyTrustRollCallSign(peer swarm.Address, expire int64, data []byte) error
}

type Service struct {
	base     swarm.Address
	streamer p2p.Streamer
	logger   logging.Logger
	topology topology.Driver
	observer MineObserver
	waits    map[uint32]*wait
	waitsMtx sync.Mutex
}

func New(streamer p2p.Streamer, logger logging.Logger, base swarm.Address) *Service {
	return &Service{
		base:     base,
		streamer: streamer,
		logger:   logger,
		waits:    make(map[uint32]*wait),
	}
}

func (s *Service) Protocol() p2p.ProtocolSpec {
	return p2p.ProtocolSpec{
		Name:    protocolName,
		Version: protocolVersion,
		StreamSpecs: []p2p.StreamSpec{
			{
				Name:    streamSign,
				Handler: s.handlerSign,
			},
			{
				Name:    streamSignRet,
				Handler: s.handlerSignRet,
			},
			{
				Name:    streamRollCall,
				Handler: s.handlerRollCall,
			},
			{
				Name:    streamRollCallSign,
				Handler: s.handlerRollCallSign,
			},
		},
	}
}

func (s *Service) SetTopology(driver topology.Driver) {
	s.topology = driver
}

func (s *Service) SetMineObserver(observer MineObserver) {
	s.observer = observer
}

func (s *Service) handlerSign(ctx context.Context, p p2p.Peer, stream p2p.Stream) (err error) {
	r := protobuf.NewReader(stream)
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			_ = stream.FullClose()
		}
	}()

	var req pb.Trust
	if err := r.ReadMsgWithContext(ctx, &req); err != nil {
		s.logger.Debugf("could not receive rollcall/sign from peer %v", p.Address)
		return fmt.Errorf("read request from peer %v: %w", p.Address, err)
	}

	if req.Expire < time.Now().Unix() {
		return fmt.Errorf("request is expire from peer %s", p.Address)
	}

	target := swarm.NewAddress(req.Stream[:32])
	if target.Equal(s.base) {
		if s.observer == nil {
			return fmt.Errorf("observer is nil")
		}
		return s.observer.NotifyTrustSignature(p.Address, req.Expire, req.Stream[32:])
	} else {
	retry:
		peer, err := s.topology.ClosestPeer(target, false, p.Address)
		if err != nil {
			return err
		}
		if peer.Equal(p.Address) {
			return p2p.ErrPeerNotFound
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSign)
		if err != nil {
			if err == p2p.ErrPeerNotFound {
				goto retry
			}
			return err
		}
		defer func() {
			if err != nil {
				_ = stream.Reset()
			} else {
				go stream.FullClose()
			}
		}()

		s.logger.Tracef("sending rollcall/sign to peer %v to %v with %s", peer, target, common.BytesToHash(req.Stream[36:68]).String())
		w := protobuf.NewWriter(stream)
		return w.WriteMsgWithContext(ctx, &req)
	}
}

func (s *Service) handlerSignRet(ctx context.Context, p p2p.Peer, stream p2p.Stream) (err error) {
	r := protobuf.NewReader(stream)
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			_ = stream.FullClose()
		}
	}()

	var req pb.Trust
	if err := r.ReadMsgWithContext(ctx, &req); err != nil {
		s.logger.Debugf("could not receive rollcall/signret from peer %v", p.Address)
		return fmt.Errorf("read request from peer %v: %w", p.Address, err)
	}

	if req.Expire < time.Now().Unix() {
		return fmt.Errorf("request is expire from peer %s", p.Address)
	}

	target := swarm.NewAddress(req.Stream[:32])
	if target.Equal(s.base) {
		id := binary.BigEndian.Uint32(req.Stream[32:36])
		s.waitsMtx.Lock()
		if w, ok := s.waits[id]; ok {
			w.C <- &req
		}
		s.waitsMtx.Unlock()
		return nil
	} else {
	retry:
		peer, err := s.topology.ClosestPeer(target, false, p.Address)
		if err != nil {
			return err
		}
		if peer.Equal(p.Address) {
			return p2p.ErrPeerNotFound
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSignRet)
		if err != nil {
			if err == p2p.ErrPeerNotFound {
				goto retry
			}
			return err
		}
		defer func() {
			if err != nil {
				_ = stream.Reset()
			} else {
				go stream.FullClose()
			}
		}()

		s.logger.Tracef("sending rollcall/signret to peer %v to %v with %s", peer, target, common.BytesToHash(req.Stream[36:68]).String())
		w := protobuf.NewWriter(stream)
		return w.WriteMsgWithContext(ctx, &req)
	}
}

func (s *Service) handlerRollCall(ctx context.Context, p p2p.Peer, stream p2p.Stream) (err error) {
	r := protobuf.NewReader(stream)
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			_ = stream.FullClose()
		}
	}()

	var req pb.Trust
	if err := r.ReadMsgWithContext(ctx, &req); err != nil {
		s.logger.Debugf("could not receive rollcall/rollcall from peer %v", p.Address)
		return fmt.Errorf("read request from peer %v: %w", p.Address, err)
	}

	if req.Expire < time.Now().Unix() {
		return fmt.Errorf("request is expire from peer %s", p.Address)
	}
	if s.observer != nil {
		return s.observer.NotifyTrustRollCall(p.Address, req.Expire, req.Stream)
	}
	return fmt.Errorf(`rollcall observer is nil`)
}

func (s *Service) handlerRollCallSign(ctx context.Context, p p2p.Peer, stream p2p.Stream) (err error) {
	r := protobuf.NewReader(stream)
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			_ = stream.FullClose()
		}
	}()

	var req pb.Trust
	if err := r.ReadMsgWithContext(ctx, &req); err != nil {
		s.logger.Debugf("could not receive rollcall/rollcallsign from peer %v", p.Address)
		return fmt.Errorf("read request from peer %v: %w", p.Address, err)
	}

	if req.Expire < time.Now().Unix() {
		return fmt.Errorf("request is expire from peer %s", p.Address)
	}

	target := swarm.NewAddress(req.Stream[:32])
	if target.Equal(s.base) {
		if s.observer != nil {
			return s.observer.NotifyTrustRollCallSign(p.Address, req.Expire, req.Stream)
		}
		return fmt.Errorf("observer is not available")
	} else {
	retry:
		peer, err := s.topology.ClosestPeer(target, false, p.Address)
		if err != nil {
			return err
		}
		if peer.Equal(p.Address) {
			return p2p.ErrPeerNotFound
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamRollCallSign)
		if err != nil {
			if err == p2p.ErrPeerNotFound {
				goto retry
			}
			return err
		}
		defer func() {
			if err != nil {
				_ = stream.Reset()
			} else {
				go stream.FullClose()
			}
		}()

		s.logger.Tracef("sending rollcall/rollcallsign to peer %v", peer)
		w := protobuf.NewWriter(stream)
		return w.WriteMsgWithContext(ctx, &req)
	}
}

func (s *Service) trustSignature(ctx context.Context, id uint32, expire int64, data []byte, target swarm.Address) error {
retry:
	peer, err := s.topology.ClosestPeer(target, false)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSign)
	if err != nil {
		if err == p2p.ErrPeerNotFound {
			goto retry
		}
		return err
	}
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			go stream.FullClose()
		}
	}()

	s.logger.Tracef("sending rollcall/sign to peer %v", peer)
	w := protobuf.NewWriter(stream)

	byts := make([]byte, 4)
	binary.BigEndian.PutUint32(byts, uint32(id))

	return w.WriteMsgWithContext(ctx, &pb.Trust{
		Expire: expire,
		Stream: append(target.Bytes(), append(byts, data...)...), // target id data
	})
}

type wait struct {
	Id uint32
	C  chan *pb.Trust
}

func (s *Service) obtainWait() *wait {
	s.waitsMtx.Lock()
	defer s.waitsMtx.Unlock()
	w := &wait{Id: uint32(time.Now().UnixNano()), C: make(chan *pb.Trust)}
	s.waits[w.Id] = w
	return w
}

func (s *Service) releaseWait(wait *wait) {
	s.waitsMtx.Lock()
	defer s.waitsMtx.Unlock()
	close(wait.C)
	delete(s.waits, wait.Id)
}

func (s *Service) TrustsSignature(ctx context.Context, expire int64, data []byte, needTrust uint64, trusts ...swarm.Address) ([]byte, error) {
	if s.topology == nil {
		return nil, fmt.Errorf("topoloy is not available")
	}

	ctx, cancal := context.WithTimeout(ctx, time.Second*20)
	defer cancal()

	// TODO To be optimized
	w := s.obtainWait()
	defer s.releaseWait(w)
	var count uint64
	for _, trust := range trusts {
		err := s.trustSignature(ctx, w.Id, expire, data, trust)
		if err != nil {
			s.logger.Debugf(`trustSignature failed %s`, err)
			continue
		}
		count += 1
	}

	if count == 0 {
		return nil, fmt.Errorf("no peer found")
	}

	if count < needTrust {
		return nil, fmt.Errorf("send a trusted node smaller than the target")
	}

	list := make([]*pb.Trust, 0, count)
	for {
		select {
		case ts := <-w.C:
			list = append(list, ts)
			needTrust -= 1
			if needTrust == 0 {
				byts := make([]byte, 0)
				for _, ts := range list {
					byts = append(byts, ts.Stream[36:]...)
				}

				return byts, nil
			}

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *Service) PushSignatures(ctx context.Context, id uint32, expire int64, data []byte, target swarm.Address, peer swarm.Address) error {
retry:
	stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSignRet)
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err == p2p.ErrPeerNotFound {
			peer, err = s.topology.ClosestPeer(target, false, peer)
			if err != nil {
				return err
			}
			goto retry
		}
		return err
	}
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			go stream.FullClose()
		}
	}()

	s.logger.Tracef("sending rollcall/signret to peer %v", target)
	w := protobuf.NewWriter(stream)

	byts := make([]byte, 4)
	binary.BigEndian.PutUint32(byts, id)

	return w.WriteMsgWithContext(ctx, &pb.Trust{
		Expire: expire,
		Stream: append(target.Bytes(), append(byts, data...)...), //target id data
	})
}

func (s *Service) PushSelfTrustSign(ctx context.Context, expire int64, data []byte, target swarm.Address) error {
retry:
	peer, err := s.topology.ClosestPeer(target, false)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamRollCallSign)
	if err != nil {
		if err == p2p.ErrPeerNotFound {
			goto retry
		}
		return err
	}
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			go stream.FullClose()
		}
	}()

	s.logger.Tracef("sending rollcall/rollcallsign to peer %v", peer)
	w := protobuf.NewWriter(stream)
	return w.WriteMsgWithContext(ctx, &pb.Trust{
		Expire: expire,
		Stream: data,
	})
}

func (s *Service) PushTrustSign(ctx context.Context, expire int64, data []byte, target swarm.Address, peer swarm.Address) error {
retry:
	stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamRollCallSign)
	if err != nil {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err == p2p.ErrPeerNotFound {
			peer, err = s.topology.ClosestPeer(target, false)
			if err != nil {
				return err
			}
			goto retry
		}
		return err
	}
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			go stream.FullClose()
		}
	}()

	s.logger.Tracef("sending rollcall/rollcallsign to peer %v", target)
	w := protobuf.NewWriter(stream)
	return w.WriteMsgWithContext(ctx, &pb.Trust{
		Expire: expire,
		Stream: data,
	})
}

func (s *Service) PushRollCall(ctx context.Context, expire int64, data []byte, skips ...swarm.Address) error {
	err := s.topology.EachPeer(func(a swarm.Address, u uint8) (stop bool, jumpToNext bool, err error) {
		for _, peer := range skips {
			if peer.Equal(a) {
				return false, false, nil
			}
		}

		stream, err := s.streamer.NewStream(ctx, a, nil, protocolName, protocolVersion, streamRollCall)
		if err != nil {
			s.logger.Debugf("PushRollCall NewStream failed: %s", err)
			return false, false, nil
		}
		defer func() {
			if err != nil {
				_ = stream.Reset()
			} else {
				go stream.FullClose()
			}
		}()

		w := protobuf.NewWriter(stream)
		err = w.WriteMsgWithContext(ctx, &pb.Trust{
			Expire: expire,
			Stream: data,
		})
		if err != nil {
			s.logger.Debugf("PushRollCall write message failed: %s", err)
		}
		return false, false, nil
	})

	if err != nil {
		s.logger.Tracef("sending rollcall/rollcall failed %v", err)
	}
	return err
}
