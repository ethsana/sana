// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rollcall

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethersphere/bee/pkg/logging"
	"github.com/ethersphere/bee/pkg/mine/rollcall/pb"
	"github.com/ethersphere/bee/pkg/p2p"
	"github.com/ethersphere/bee/pkg/p2p/protobuf"
	"github.com/ethersphere/bee/pkg/swarm"
	"github.com/ethersphere/bee/pkg/topology"
)

const (
	protocolName    = "rollcall"
	protocolVersion = "1.0.0"
	streamName      = "rollcall"
)

var (
	// ErrThresholdTooLow says that the proposed payment threshold is too low for even a single reserve.
	ErrThresholdTooLow = errors.New("threshold too low")
)

type CertificateObserver interface {
	NotifyCertificate(peer swarm.Address, data []byte) ([]byte, error)
}

type Service struct {
	base           swarm.Address
	streamer       p2p.Streamer
	logger         logging.Logger
	topologyDriver topology.Driver
	observer       CertificateObserver
}

func New(streamer p2p.Streamer, logger logging.Logger, base swarm.Address) *Service {
	return &Service{
		base:     base,
		streamer: streamer,
		logger:   logger,
	}
}

func (s *Service) Protocol() p2p.ProtocolSpec {
	return p2p.ProtocolSpec{
		Name:    protocolName,
		Version: protocolVersion,
		StreamSpecs: []p2p.StreamSpec{
			{
				Name:    streamName,
				Handler: s.handler,
			},
		},
	}
}

func (s *Service) SetTopology(driver topology.Driver) {
	s.topologyDriver = driver
}

func (s *Service) SetCertificateObserver(observer CertificateObserver) {
	s.observer = observer
}

func (s *Service) handler(ctx context.Context, p p2p.Peer, stream p2p.Stream) (err error) {
	w, r := protobuf.NewWriterAndReader(stream)
	defer func() {
		if err != nil {
			_ = stream.Reset()
		} else {
			_ = stream.FullClose()
		}
	}()

	if s.observer == nil {
		s.logger.Debugf("certificateObserver is nil")
		return fmt.Errorf("certificateObserver is nil")
	}

	var req pb.Certificate
	if err := r.ReadMsgWithContext(ctx, &req); err != nil {
		s.logger.Debugf("could not receive rollcall from peer %v", p.Address)
		return fmt.Errorf("read request from peer %v: %w", p.Address, err)
	}

	addr := swarm.NewAddress(req.Address)
	if addr.Equal(s.base) {
		data, err := s.observer.NotifyCertificate(swarm.NewAddress(req.Data), nil)
		if err != nil {
			return err
		}
		return w.WriteMsgWithContext(ctx, &pb.Certificate{Address: req.Address, Data: data})
	} else {
		peer, err := s.topologyDriver.ClosestPeer(addr, false, p.Address)
		if err != nil {
			return err
		}

		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamName)
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

		s.logger.Tracef("sending rollcall to peer %v", peer)
		w, r := protobuf.NewWriterAndReader(stream)
		err = w.WriteMsgWithContext(ctx, &req)
		if err != nil {
			return err
		}

		var req pb.Certificate
		if err := r.ReadMsgWithContext(ctx, &req); err != nil {
			s.logger.Debugf("could not receive rollcall from peer %v", peer)
			return fmt.Errorf("read request from peer %v: %w", peer, err)
		}

		return w.WriteMsgWithContext(ctx, &req)
	}
}

func (s *Service) Certificate(ctx context.Context, peer_ swarm.Address, data []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if s.topologyDriver != nil {
		peer, err := s.topologyDriver.ClosestPeer(peer_, false)
		if err != nil {
			return nil, err
		}

		stream, err := s.streamer.NewStream(ctx, peer, nil, protocolName, protocolVersion, streamName)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				_ = stream.Reset()
			} else {
				go stream.FullClose()
			}
		}()

		s.logger.Tracef("sending rollcall to peer %v", peer)
		w, r := protobuf.NewWriterAndReader(stream)
		err = w.WriteMsgWithContext(ctx, &pb.Certificate{
			Address: peer_.Bytes(),
			Data:    data[:],
		})
		if err != nil {
			return nil, err
		}

		var req pb.Certificate
		if err := r.ReadMsgWithContext(ctx, &req); err != nil {
			s.logger.Debugf("could not receive rollcall from peer %v", peer)
			return nil, fmt.Errorf("read request from peer %v: %w", peer, err)
		}

		return req.Data, err
	}
	return nil, fmt.Errorf(`topology is nil`)
}
