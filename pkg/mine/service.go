// Copyright 2021 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mine

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/crypto"
	"github.com/ethsana/sana/pkg/logging"
	"github.com/ethsana/sana/pkg/sctx"
	"github.com/ethsana/sana/pkg/settlement/swap/erc20"
	"github.com/ethsana/sana/pkg/storage"
	"github.com/ethsana/sana/pkg/swarm"
	"github.com/ethsana/sana/pkg/transaction"
)

const (
	minePrefix     = "mine"
	mineDepositKey = "mine_deposit"
)

var (
	// ErrNotFound is the error returned when issuer with given batch ID does not exist.
	ErrNotFound = errors.New("not found")
)

// 50000 SANA
var defaultMineDeposit = new(big.Int).Mul(new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil), big.NewInt(5e4))

// Service is the miner service interface.
type Service interface {
	Start(startBlock uint64) (<-chan struct{}, error)
	Close() error
	SetTrust(rollcall Trust)

	NotifyTrustSignature(pper swarm.Address, id, op int32, expire int64, data []byte) error
	NotifyTrustRollCall(peer swarm.Address, expire int64, data []byte) error
	NotifyTrustRollCallSign(peer swarm.Address, expire int64, data []byte) error

	Status(ctx context.Context) (bool, *big.Int, *big.Int, *big.Int, error)
	Withdraw(ctx context.Context) (common.Hash, error)
	CashDeposit(ctx context.Context) (common.Hash, error)
}

// service handles mine
type service struct {
	base     swarm.Address
	signer   crypto.Signer
	nodes    NodeService
	contract MineContract
	trust    Trust
	logger   logging.Logger

	opt *Options

	height    uint64
	heightMtx sync.Mutex
	quit      chan struct{}
	wg        sync.WaitGroup
}

type Options struct {
	Store              storage.StateStorer
	Backend            transaction.Backend
	TransactionService transaction.Service
	OverlayEthAddress  common.Address
	DeployGasPrice     string
}

// NewService constructs a new Service.
func NewService(
	base swarm.Address,
	contract MineContract,
	nodes NodeService,
	signer crypto.Signer,
	logger logging.Logger,
	warmupTime time.Duration,
	opt Options,
) Service {
	return &service{
		base:     base,
		signer:   signer,
		contract: contract,
		nodes:    nodes,
		logger:   logger,
		opt:      &opt,
		quit:     make(chan struct{}),
	}
}

func (s *service) NotifyTrustSignature(peer swarm.Address, id int32, op int32, expire int64, data []byte) error {
	node := common.BytesToHash(data[:32])

	addr, err := recoverSignAddress(data[32:], op, expire, node)
	if err != nil {
		return err
	}

	owner, err := s.nodes.MineAddress(node, s.contract)
	if err != nil {
		return err
	}

	if owner == addr {
		signature, err := signLocalTrustData(s.signer, node, expire)
		if err != nil {
			return err
		}

		ctx, cancal := context.WithTimeout(context.Background(), time.Minute)
		defer cancal()
		return s.trust.PushSignatures(ctx, id, op, expire, signature, swarm.NewAddress(node.Bytes()), peer)
	}
	return fmt.Errorf("invalid signature")
}

func (s *service) NotifyTrustRollCall(peer swarm.Address, expire int64, data []byte) error {
	height := binary.BigEndian.Uint64(data[32:])
	s.heightMtx.Lock()
	if s.height >= height {
		s.heightMtx.Unlock()
		s.logger.Debugf("rollcall message already handler")
		return nil
	}
	s.height = height
	s.heightMtx.Unlock()

	ctx, cancal := context.WithTimeout(context.Background(), time.Minute)
	defer cancal()

	if !s.nodes.TrustOf(s.base) {
		signature, err := signLocalTrustData(s.signer, common.BytesToHash(data[:32]), expire)
		if err != nil {
			return err
		}

		err = s.trust.PushTrustSign(ctx, expire, append(append(append(data[:32], s.base.Bytes()...), signature...), data[32:]...), swarm.NewAddress(data[32:]), peer)
		if err != nil {
			return err
		}
		return s.trust.PushRollCall(ctx, expire, data, peer)
	}
	return nil
}

func (s *service) NotifyTrustRollCallSign(_ swarm.Address, expire int64, data []byte) error {
	node := common.BytesToHash(data[32:64])

	addr, err := recoverSignAddress(data[64:129], common.BytesToHash(s.base.Bytes()), expire)
	if err != nil {
		return err
	}

	owner, err := s.nodes.MineAddress(node, s.contract)
	if err != nil {
		return err
	}

	if owner == addr {
		height := binary.BigEndian.Uint64(data[129:])
		return s.nodes.UpdateNodeLastBlock(swarm.NewAddress(node.Bytes()), height)
	}
	return fmt.Errorf("invalid signature")
}

func (s *service) mortgageMiner(ctx context.Context) error {
	o := s.opt

	if o.DeployGasPrice != "" {
		gasPrice, ok := new(big.Int).SetString(o.DeployGasPrice, 10)
		if !ok {
			return fmt.Errorf("deploy gas price \"%s\" cannot be parsed", o.DeployGasPrice)
		}
		ctx = sctx.SetGasPrice(ctx, gasPrice)
	}

	var txHash common.Hash
	err := o.Store.Get(mineDepositKey, &txHash)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	if err == storage.ErrNotFound {
		erc20Address, err := s.contract.Token(ctx)
		if err != nil {
			return err
		}

		erc20Service := erc20.New(o.Backend, o.TransactionService, erc20Address)
		s.logger.Info("no mine deposit tx found, deposit now.")

		gasPrice, err := o.Backend.SuggestGasPrice(ctx)
		if err != nil {
			return err
		}

		minimumEth := gasPrice.Mul(gasPrice, big.NewInt(250000))

		ethBalance, err := o.Backend.BalanceAt(ctx, o.OverlayEthAddress, nil)
		if err != nil {
			return err
		}
		if ethBalance.Cmp(minimumEth) < 0 {
			return fmt.Errorf("insufficient ETH and gas payment")
		}

		erc20Balance, err := erc20Service.BalanceOf(ctx, o.OverlayEthAddress)
		if err != nil {
			return err
		}
		if erc20Balance.Cmp(defaultMineDeposit) < 0 {
			return fmt.Errorf("ETH address SANA amount is less than 50000 SANA")
		}

		lockupAddress, err := s.contract.Lockup(ctx)
		if err != nil {
			return err
		}

		amount, err := erc20Service.Allowance(ctx, o.OverlayEthAddress, lockupAddress)
		if err != nil {
			return err
		}

		if amount.Cmp(defaultMineDeposit) < 0 {
			hash, err := erc20Service.Approve(ctx, lockupAddress, defaultMineDeposit)
			if err != nil {
				return err
			}

			err = erc20Service.WaitForApprove(ctx, hash)
			if err != nil {
				return err
			}
		}

		txHash, err = s.contract.Deposit(ctx, common.BytesToHash(s.base.Bytes()))
		if err != nil {
			return err
		}

		s.logger.Infof("mine deposit in transaction %x", txHash)
		err = o.Store.Put(mineDepositKey, txHash)
		if err != nil {
			return err
		}
	} else {
		s.logger.Infof("waiting for mine deposit in transaction %x", txHash)
	}
	defer o.Store.Delete(mineDepositKey)
	return s.contract.WaitForDeposit(ctx, txHash)
}

func (s *service) checkWorkingWorker() (bool, error) {
	ctx, cancal := context.WithTimeout(context.Background(), time.Second*20)
	defer cancal()

	node := common.BytesToHash(s.base.Bytes())
	work, err := s.contract.IsWorking(ctx, node)
	if err != nil {
		return false, err
	}

	if work {
		return true, nil
	}

	// check deposit
	deposit, err := s.contract.CheckDeposit(ctx, node)
	if err != nil {
		return false, err
	}

	if !deposit {
		quit := make(chan struct{})
		go func() {
			defer close(quit)
			err = s.mortgageMiner(ctx)
		}()

		select {
		case <-quit:
		case <-ctx.Done():
			return false, ctx.Err()
		}

		if err != nil {
			return false, err
		}
	}

	trusts := s.nodes.TrustAddress(func(a swarm.Address) bool { return !s.base.Equal(a) })
	if len(trusts) == 0 {
		return false, fmt.Errorf("no trust nodes")
	}

	needTrust, err := s.contract.ValidateTrusts(ctx)
	if err != nil {
		return false, err
	}

	if needTrust.Cmp(new(big.Int)) <= 0 {
		return false, fmt.Errorf("need trust nums is zero")
	}

	expire := time.Now().Add(time.Minute).Unix()
	signature, err := signLocalTrustData(s.signer, 1, expire, node)
	if err != nil {
		return false, err
	}

	var (
		signatures []byte
	)
	quit := make(chan struct{})
	go func() {
		defer close(quit)
		signatures, err = s.trust.TrustsSignature(ctx, 1, expire, append(s.base.Bytes(), signature...), needTrust.Uint64(), trusts...)
	}()

	select {
	case <-quit:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	if err != nil {
		return false, err
	}

	if len(signatures)/65 != int(needTrust.Uint64()) {
		return false, fmt.Errorf("insufficient signatures")
	}

	hash, err := s.contract.Active(ctx, node, new(big.Int).SetInt64(expire), signatures)
	if err != nil {
		return false, err
	}

	err = s.contract.WaitForActive(ctx, hash)
	return err == nil, err
}

func (s *service) checkExpireMiners() error {
	ctx, cancal := context.WithTimeout(context.Background(), time.Second*20)
	defer cancal()

	miners, err := s.nodes.ExpireMiners()
	if err != nil {
		return err
	}

	// dict := make(map[string][]string)
	// for _, addr := range miners {
	// 	eth, _ := s.nodes.MineAddress(common.BytesToHash(addr.Bytes()), s.contract)
	// 	dict[eth.String()] = append(dict[eth.String()], addr.String())
	// }
	// for eth, nodes := range dict {
	// 	fmt.Println(`>>>`, eth)
	// 	for _, v := range nodes {
	// 		fmt.Println(`>>>`, "\t", v)
	// 	}
	// }
	// fmt.Printf("\033[0;31;40m  >>>>>>>>>>>>>>>>>>>>>>>>> expire size %v \033[0m\n", len(miners))
	// return nil

	if len(miners) == 0 {
		return nil
	}

	needTrust, err := s.contract.ValidateTrusts(ctx)
	if err != nil {
		return err
	}

	if needTrust.Cmp(big.NewInt(1)) == 0 {
		// self	sign
		expire := time.Now().Add(time.Minute).Unix()
		for _, node := range miners {
			signature, err := signLocalTrustData(s.signer, node, expire)
			if err != nil {
				s.logger.Infof("inaction address %s signature failed at %s", node.String(), err)
				continue
			}
			hash, err := s.contract.Inaction(ctx, common.BytesToHash(node.Bytes()), big.NewInt(expire), signature)
			if err != nil {
				addr, _ := s.nodes.MineAddress(common.BytesToHash(node.Bytes()), s.contract)
				s.logger.Infof("inaction address %s/%s sendtransaction failed at %s", node.String(), addr.String(), err)
				continue
			}

			s.nodes.UpdateNodeInactionTxHash(node, hash)
			s.logger.Infof("inaction address %s transaction %s", node.String(), hash.String())
			// TODO second send transaction
		}
	}
	// else {
	// 	// TODO multi trust signature
	// }

	return nil
}

func (s *service) checkSelfTrustRollCallSign(height uint64) error {
	// s.heightMtx.Lock()
	// if s.height >= height {
	// 	s.heightMtx.Unlock()
	// 	s.logger.Debugf("rollcall message already handler")
	// 	return fmt.Errorf("already handler")
	// }
	// s.height = height
	// s.heightMtx.Unlock()

	trusts := s.nodes.TrustAddress(func(a swarm.Address) bool { return !a.Equal(s.base) })
	if len(trusts) == 0 {
		return fmt.Errorf("no trust nodes")
	}

	ctx, cancal := context.WithTimeout(context.Background(), time.Minute)
	defer cancal()

	expire := time.Now().Add(time.Minute).Unix()
	byts := make([]byte, 8)
	binary.BigEndian.PutUint64(byts, height)
	for _, addr := range trusts {
		signature, err := signLocalTrustData(s.signer, addr.Bytes(), expire)
		if err != nil {
			s.logger.Debugf("rollcal to %s trust node signature failed: %s", addr.String(), err)
			continue
		}
		err = s.trust.PushSelfTrustSign(ctx, expire, append(append(append(addr.Bytes(), s.base.Bytes()...), signature...), byts...), addr)
		if err != nil {
			s.logger.Debugf("self push to %s trust sign failed: %s", addr.String(), err)
		}
	}
	return nil
}

func (s *service) manange() {
	defer s.wg.Done()

	c, unsubscribe := s.nodes.SubscribeRollCall()
	defer unsubscribe()

	s.logger.Info("mine worker starting.")

	timer := time.NewTimer(0)
	defer timer.Stop()

	var expireChan <-chan time.Time

	for {
		select {
		case height := <-c:
			if s.nodes.TrustOf(s.base) {
				s.logger.Infof("mine: start to detect node online")
				expire := time.Now().Add(time.Minute).Unix()

				byts := make([]byte, 8)
				binary.BigEndian.PutUint64(byts, height)
				err := s.trust.PushRollCall(context.Background(), expire, append(s.base.Bytes(), byts...))
				if err != nil {
					s.logger.Infof("push to detect online message failed: %s", err)
				}

				// check expire nodes
				expireChan = time.After(time.Second)
			}
			err := s.checkSelfTrustRollCallSign(height)
			if err != nil {
				s.logger.Debugf("self check rollcall sign failed: at %s", err)
			}

		case <-expireChan:
			err := s.checkExpireMiners()
			if err != nil {
				s.logger.Infof("inaction expire miner failed at %s", err)
			}

		case <-timer.C:
			ok, err := s.checkWorkingWorker()
			if err != nil {
				s.logger.Infof("check mine working failed at %s", err)
			}
			if !ok {
				timer.Reset(time.Second * 10)
			} else {
				timer.Reset(time.Minute * 5)
				s.logger.Infof("the overlay address %s mining", s.base.String())
			}

		case <-s.quit:
			return
		}
	}
}

func (s *service) SetTrust(trust Trust) {
	s.trust = trust
}

func (s *service) NotifyCertificate(peer swarm.Address, data []byte) ([]byte, error) {
	// TODO check peer is active
	deadline := big.NewInt(time.Now().Add(time.Minute).Unix())
	deadbyts := deadline.Bytes()
	byts := make([]byte, 64)
	copy(byts[:32], peer.Bytes())
	copy(byts[32:], common.BigToHash(deadline).Bytes())
	byts, err := crypto.LegacyKeccak256(byts)
	if err != nil {
		return nil, err
	}
	byts, err = s.signer.Sign(byts)
	if err != nil {
		return nil, err
	}
	return append(byts, deadbyts...), nil
}

func (s *service) Status(ctx context.Context) (bool, *big.Int, *big.Int, *big.Int, error) {
	node := common.BytesToHash(s.base.Bytes())
	work, err := s.contract.IsWorking(ctx, node)
	if err != nil {
		return false, nil, nil, nil, err
	}
	withdraw, err := s.contract.MinersWithdraw(ctx, node)
	if err != nil {
		return false, nil, nil, nil, err
	}
	reward, err := s.contract.Reward(ctx, node)
	if err != nil {
		return false, nil, nil, nil, err
	}
	expire, err := s.contract.ExpireOf(ctx, node)
	return work, withdraw, reward, expire, err
}

func (s *service) Withdraw(ctx context.Context) (common.Hash, error) {
	node := common.BytesToHash(s.base.Bytes())
	trusts := s.nodes.TrustAddress(func(a swarm.Address) bool { return !a.Equal(s.base) })
	if len(trusts) == 0 {
		return common.Hash{}, fmt.Errorf("no trust nodes")
	}

	needTrust, err := s.contract.ValidateTrusts(ctx)
	if err != nil {
		return common.Hash{}, err
	}

	if needTrust.Cmp(new(big.Int)) <= 0 {
		return common.Hash{}, fmt.Errorf("need trust nums is zero")
	}

	expire := time.Now().Add(time.Minute).Unix()
	signature, err := signLocalTrustData(s.signer, 1, expire, node)
	if err != nil {
		return common.Hash{}, err
	}

	var (
		signatures []byte
	)
	quit := make(chan struct{})
	go func() {
		defer close(quit)
		signatures, err = s.trust.TrustsSignature(ctx, 1, expire, append(s.base.Bytes(), signature...), needTrust.Uint64(), trusts...)
	}()

	select {
	case <-quit:
	case <-ctx.Done():
		return common.Hash{}, ctx.Err()
	}
	if err != nil {
		return common.Hash{}, err
	}

	if len(signatures)/65 != int(needTrust.Uint64()) {
		return common.Hash{}, fmt.Errorf("insufficient signatures")
	}

	hash, err := s.contract.Withdraw(ctx, node, new(big.Int).SetInt64(expire), signatures)
	if err != nil {
		return common.Hash{}, err
	}

	return hash, err
}

func (s *service) CashDeposit(ctx context.Context) (common.Hash, error) {
	node := common.BytesToHash(s.base.Bytes())

	expire, err := s.contract.ExpireOf(ctx, node)
	if err != nil {
		return common.Hash{}, err
	}

	if expire.Int64() >= time.Now().Unix() {
		return common.Hash{}, fmt.Errorf("the deposit is not due")
	}

	return s.contract.CashDeposit(ctx, node)
}

func (s *service) Start(startBlock uint64) (<-chan struct{}, error) {
	s.wg.Add(1)
	go s.manange()

	return s.nodes.Start(startBlock)
}

// Close saves all the active stamp issuers to statestore.
func (s *service) Close() error {
	s.logger.Info("mine shutting down")
	close(s.quit)
	cc := make(chan struct{})
	go func() {
		defer close(cc)
		s.wg.Wait()
	}()
	select {
	case <-cc:
	case <-time.After(10 * time.Second):
		s.logger.Warning("mine shutting down with running goroutines")
	}
	return nil
}
