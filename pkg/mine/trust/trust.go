// Copyright 2020 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trust

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	streamRollCall     = "rollcall"
	streamRollCallSign = "rollcallsign"
)

type MineObserver interface {
	NotifyTrustSignature(id, op int32, expire int64, data []byte) error
	NotifyTrustRollCall(peer swarm.Address, op int32, expire int64, data []byte) error
	NotifyTrustRollCallSign(op int32, expire int64, data []byte) error
}

type Service struct {
	base     swarm.Address
	streamer p2p.Streamer
	logger   logging.Logger
	topology topology.Driver
	observer MineObserver
	waits    map[int32]*wait
	waitsMtx sync.Mutex
}

func New(streamer p2p.Streamer, logger logging.Logger, base swarm.Address) *Service {
	return &Service{
		base:     base,
		streamer: streamer,
		logger:   logger,
		waits:    make(map[int32]*wait),
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

	var req pb.TrustSign
	if err := r.ReadMsgWithContext(ctx, &req); err != nil {
		s.logger.Debugf("could not receive rollcall/sign from peer %v", p.Address)
		return fmt.Errorf("read request from peer %v: %w", p.Address, err)
	}

	if req.Expire < time.Now().Unix() {
		return fmt.Errorf("request is expire from peer %s", p.Address)
	}

	target := swarm.NewAddress(req.Peer)
	if target.Equal(s.base) {
		if req.Result {
			// TODO To be optimized
			s.waitsMtx.Lock()
			if w, ok := s.waits[req.Id]; ok {
				w.C <- &req
			}
			s.waitsMtx.Unlock()

		} else if s.observer != nil {
			err = s.observer.NotifyTrustSignature(req.Id, req.Op, req.Expire, req.Data)
			return err
		}

		return nil
	} else {
		peer, err := s.topology.ClosestPeer(target, false, p.Address)
		if err != nil {
			return err
		}

		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSign)
		if err != nil {
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
		return s.observer.NotifyTrustRollCall(p.Address, 0, req.Expire, req.Stream)
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
			return s.observer.NotifyTrustRollCallSign(0, req.Expire, req.Stream)
		}
		return fmt.Errorf("observer is not available")
	} else {
		peer, err := s.topology.ClosestPeer(target, false, p.Address)
		if err != nil {
			return err
		}

		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSign)
		if err != nil {
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

func (s *Service) trustSignature(ctx context.Context, id, op int32, expire int64, data []byte, target swarm.Address) error {
	peer, err := s.topology.ClosestPeer(target, false)
	if err != nil {
		return err
	}
	stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSign)
	if err != nil {
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
	return w.WriteMsgWithContext(ctx, &pb.TrustSign{
		Id:     id,
		Op:     op,
		Data:   data,
		Peer:   target.Bytes(),
		Expire: expire,
	})
}

type wait struct {
	Id int32
	C  chan *pb.TrustSign
}

func (s *Service) obtainWait() *wait {
	s.waitsMtx.Lock()
	defer s.waitsMtx.Unlock()
	w := &wait{Id: int32(time.Now().UnixNano()), C: make(chan *pb.TrustSign)}
	s.waits[w.Id] = w
	return w
}

func (s *Service) releaseWait(wait *wait) {
	s.waitsMtx.Lock()
	defer s.waitsMtx.Unlock()
	if _, ok := s.waits[wait.Id]; ok {
		delete(s.waits, wait.Id)
	}
}

func (s *Service) TrustsSignature(ctx context.Context, op int32, expire int64, data []byte, peers ...swarm.Address) ([]byte, error) {
	ctx, cancal := context.WithTimeout(ctx, time.Second*20)
	defer cancal()

	// TODO To be optimized
	if s.topology != nil {
		w := s.obtainWait()
		defer s.releaseWait(w)
		var count uint
		for _, peer := range peers {
			err := s.trustSignature(ctx, w.Id, op, expire, data, peer)
			if err != nil {
				s.logger.Debugf(`trustSignature failed %s`, err)
				continue
			}
			count += 1
		}

		if count == 0 {
			return nil, fmt.Errorf("no peer found")
		}

		list := make([]*pb.TrustSign, 0, count)
		for {
			select {
			case ts := <-w.C:
				list = append(list, ts)
				count -= 1
				if count == 0 {
					byts := make([]byte, 0)
					for _, ts := range list {
						byts = append(byts, ts.Data...)
					}

					return byts, nil
				}

			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

	}
	return nil, fmt.Errorf(`topology is nil`)
}

func (s *Service) PushSignatures(ctx context.Context, id, op int32, expire int64, data []byte, target swarm.Address) error {
	if s.topology != nil {
		peer, err := s.topology.ClosestPeer(target, false)
		if err != nil {
			return err
		}
		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamSign)
		if err != nil {
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
		return w.WriteMsgWithContext(ctx, &pb.TrustSign{
			Id:     id,
			Op:     op,
			Data:   data,
			Peer:   target.Bytes(),
			Expire: expire,
			Result: true,
		})
	}
	return nil
}

func (s *Service) PushTrustSign(ctx context.Context, op int32, expire int64, data []byte, target swarm.Address) error {
	if s.topology != nil {
		peer, err := s.topology.ClosestPeer(target, false)
		if err != nil {
			return err
		}
		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamRollCallSign)
		if err != nil {
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
	return nil
}

func (s *Service) PushRollCall(ctx context.Context, op int32, expire int64, data []byte, skips ...swarm.Address) error {
	if s.topology != nil {
		err := s.topology.EachPeer(func(a swarm.Address, u uint8) (stop bool, jumpToNext bool, err error) {
			for _, peer := range skips {
				if peer.Equal(a) {
					return false, true, nil
				}
			}

			stream, err := s.streamer.NewStream(ctx, a, nil, protocolName, protocolVersion, streamRollCall)
			if err != nil {
				return false, true, err
			}
			if err != nil {
				return false, true, err
			}
			defer func() {
				if err != nil {
					_ = stream.Reset()
				} else {
					go stream.FullClose()
				}
			}()

			w := protobuf.NewWriter(stream)
			return false, true, w.WriteMsgWithContext(ctx, &pb.Trust{
				Expire: expire,
				Stream: data,
			})
		})

		if err != nil {
			s.logger.Tracef("sending rollcall/rollcall failed %v", err)
		}
		return nil
	}
	return fmt.Errorf("topoloy is not available")
}