package syncer

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethsana/sana/pkg/logging"
)

const (
	blockPage = 5000 // how many blocks to sync every time we page
	tailSize  = 4    // how many blocks to tail from the tip of the chain
)

type BlockHeightContractFilterer interface {
	bind.ContractFilterer
	BlockNumber(context.Context) (uint64, error)
}

type Shutdowner interface {
	Shutdown(context.Context) error
}

type EventUpdater interface {
	ProcessEvent(e types.Log) error
	UpdateBlockNumber(blockNumber uint64) error
	TransactionStart() error
	TransactionEnd() error
}

type Sync struct {
	From       uint64
	Updater    EventUpdater
	FilterLogs func(from, to *big.Int) ethereum.FilterQuery
}

type Service interface {
	AddSync(sync *Sync)
	Worker() <-chan struct{}
}

type service struct {
	logger    logging.Logger
	ev        BlockHeightContractFilterer
	blockTime uint64

	syncMtx sync.Mutex
	syncs   []*Sync

	quit       chan struct{}
	wg         sync.WaitGroup
	shutdowner Shutdowner
}

func New(
	logger logging.Logger,
	ev BlockHeightContractFilterer,
	blockTime uint64,
	shutdowner Shutdowner,
) *service {
	return &service{
		logger:     logger,
		ev:         ev,
		blockTime:  blockTime,
		quit:       make(chan struct{}),
		shutdowner: shutdowner,
	}
}

func (s *service) AddSync(sync *Sync) {
	s.syncMtx.Lock()
	defer s.syncMtx.Unlock()
	s.syncs = append(s.syncs, sync)
}

func (s *service) Worker() <-chan struct{} {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-s.quit
		cancel()
	}()

	chainUpdateInterval := time.Duration(s.blockTime) / 2

	synced := make(chan struct{})
	closeOnce := new(sync.Once)
	paged := make(chan struct{}, 1)
	paged <- struct{}{}

	listenf := func() (bool, error) {
		defer s.wg.Done()
		for {
			select {
			case <-paged:
				// if we paged then it means there's more things to sync on
			case <-time.After(chainUpdateInterval):
			case <-s.quit:
				return true, nil
			}

			height, err := s.ev.BlockNumber(ctx)
			if err != nil {
				return false, err
			}

			if height < tailSize {
				// in a test blockchain there might be not be enough blocks yet
				continue
			}

			if len(s.syncs) == 0 {
				continue
			}

			// consider to-tailSize as the "latest" block we need to sync to
			height = height - tailSize

			var more bool
			for _, sync := range s.syncs {
				to, from := height, sync.From

				if to < from {
					continue
				}

				if to-from > blockPage {
					more = true
					to = from + blockPage
				}

				events, err := s.ev.FilterLogs(ctx, sync.FilterLogs(big.NewInt(int64(from)), big.NewInt(int64(to))))
				if err != nil {
					return false, err
				}

				if err := sync.Updater.TransactionStart(); err != nil {
					return true, err
				}

				for _, e := range events {
					err = sync.Updater.UpdateBlockNumber(e.BlockNumber)
					if err != nil {
						return true, err
					}
					if err = sync.Updater.ProcessEvent(e); err != nil {
						return true, err
					}
				}

				err = sync.Updater.UpdateBlockNumber(to)
				if err != nil {
					return true, err
				}

				if err := sync.Updater.TransactionEnd(); err != nil {
					return true, err
				}

				sync.From = to + 1
			}

			if more {
				paged <- struct{}{}
			} else {
				closeOnce.Do(func() { close(synced) })
			}
		}
	}

	go func() {
		for {
			s.wg.Add(1)
			stop, err := listenf()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					// context cancelled is returned on shutdown,
					// therefore we do nothing here
					return
				}

				s.logger.Errorf("syncing event fail: %v", err)
				if s.shutdowner != nil && stop {
					err = s.shutdowner.Shutdown(context.Background())
					if err != nil {
						s.logger.Errorf("failed shutting down node: %v", err)
					}
					return
				}
			}
		}
	}()

	return synced
}

func (s *service) Close() error {
	s.logger.Info("syncer shutting down")
	close(s.quit)
	done := make(chan struct{})

	go func() {
		defer close(done)
		s.wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		return errors.New("listener closed with running goroutines")
	}
	return nil
}
