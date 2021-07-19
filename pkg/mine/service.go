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

const (
	balanceCheckBackoffDuration = 20 * time.Second
	balanceCheckMaxRetries      = 10
)

const ()

var (
	// ErrNotFound is the error returned when issuer with given batch ID does not exist.
	ErrNotFound = errors.New("not found")
)

// Service is the miner service interface.
type Service interface {
	Start(startBlock uint64) (<-chan struct{}, error)
	Close() error
	Deposit(store storage.StateStorer, swapBackend transaction.Backend, erc20Service erc20.Service, overlayEthAddress common.Address, deployGasPrice string) error
	SetTrust(rollcall Trust)

	NotifyTrustSignature(id, op int32, expire int64, data []byte) error
	NotifyTrustRollCall(peer swarm.Address, op int32, expire int64, data []byte) error
	NotifyTrustRollCallSign(op int32, expire int64, data []byte) error

	Status(ctx context.Context) (bool, *big.Int, *big.Int, *big.Int, error)
	Withdraw(ctx context.Context) (common.Hash, error)
	CashDeposit(ctx context.Context) (common.Hash, error)
}

// service handles postage batches
// stores the active batches.
type service struct {
	base     swarm.Address
	signer   crypto.Signer
	nodes    NodeService
	contract MineContract
	trust    Trust
	logger   logging.Logger

	quit chan struct{}
	wg   sync.WaitGroup
}

// NewService constructs a new Service.
func NewService(
	base swarm.Address,
	contract MineContract,
	nodes NodeService,
	signer crypto.Signer,
	logger logging.Logger,
	warmupTime time.Duration,
) (Service, error) {
	s := &service{
		base:     base,
		signer:   signer,
		contract: contract,
		nodes:    nodes,
		logger:   logger,
		quit:     make(chan struct{}),
	}

	return s, nil
}

func (s *service) NotifyTrustSignature(id int32, op int32, expire int64, data []byte) error {
	node := common.BytesToHash(data[:32])

	addr, err := recoverSignAddress(data[32:], op, expire, node)
	if err != nil {
		return err
	}

	owner, err := s.contract.MinersReceived(context.Background(), node)
	if err != nil {
		return err
	}

	if owner == addr {
		signature, err := signLocalTrustData(s.signer, node, expire)
		if err != nil {
			return err
		}
		return s.trust.PushSignatures(context.Background(), id, op, expire, signature, swarm.NewAddress(node[:]))
	}
	return fmt.Errorf("invalid signature")
}

func (s *service) NotifyTrustRollCall(peer swarm.Address, op int32, expire int64, data []byte) error {
	signature, err := signLocalTrustData(s.signer, common.BytesToHash(data[:32]), expire)
	if err != nil {
		return err
	}

	err = s.trust.PushTrustSign(context.Background(), op, expire, append(append(append(data[:32], s.base.Bytes()...), signature...), data[32:]...), swarm.NewAddress(data[:32]))
	if err != nil {
		return err
	}
	return s.trust.PushRollCall(context.Background(), op, expire, data, peer)
}

func (s *service) NotifyTrustRollCallSign(op int32, expire int64, data []byte) error {
	node := common.BytesToHash(data[32:64])

	addr, err := recoverSignAddress(data[64:129], common.BytesToHash(s.base.Bytes()), expire)
	if err != nil {
		return err
	}

	owner, err := s.contract.MinersReceived(context.Background(), node)
	if err != nil {
		return err
	}

	if owner == addr {
		height := binary.BigEndian.Uint64(data[129:])
		return s.nodes.UpdateNodeLastBlock(swarm.NewAddress(node.Bytes()), height)
	}
	return fmt.Errorf("invalid signature")
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

	trusts, err := s.nodes.TrustAddress(func(a swarm.Address) bool { return !s.base.Equal(a) })
	if err != nil {
		return false, err
	}

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
		signatures, err = s.trust.TrustsSignature(ctx, 1, expire, append(s.base.Bytes(), signature...), trusts...)
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
			ok, err := s.contract.IsWorking(ctx, common.BytesToHash(node.Bytes()))
			if err != nil {
				s.logger.Infof("check address %s working failed at %s", node.String(), err)
			}
			if !ok {
				continue
			}

			signature, err := signLocalTrustData(s.signer, node, expire)
			if err != nil {
				s.logger.Infof("inaction address %s signature failed at %s", node.String(), err)
				continue
			}
			hash, err := s.contract.Inaction(ctx, common.BytesToHash(node.Bytes()), big.NewInt(expire), signature)
			if err != nil {
				s.logger.Infof("inaction address %s sendtransaction failed at %s", node.String(), err)
				continue
			}
			s.logger.Infof("inaction address %s transaction %s", node.String(), hash)
			// TODO second send transaction
		}
	} else {
		// TODO multi trust signature
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
				err := s.trust.PushRollCall(context.Background(), 2, expire, append(s.base.Bytes(), byts...))
				if err != nil {
					s.logger.Infof("push to detect online message failed: %s", err)
				}

				expireChan = time.After(time.Second * 20)
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

func (s *service) Deposit(
	store storage.StateStorer,
	swapBackend transaction.Backend,
	erc20Service erc20.Service,
	overlayEthAddress common.Address,
	deployGasPrice string) error {

	ctx := context.Background()

	node := common.BytesToHash(s.base.Bytes())

	ok, err := s.contract.CheckDeposit(ctx, node)
	if err != nil {
		return err
	}

	if ok {
		s.logger.Infof("using the address %v has deposit.", s.base.String())
		return nil
	}

	if deployGasPrice != "" {
		gasPrice, ok := new(big.Int).SetString(deployGasPrice, 10)
		if !ok {
			return fmt.Errorf("deploy gas price \"%s\" cannot be parsed", deployGasPrice)
		}
		ctx = sctx.SetGasPrice(ctx, gasPrice)
	}

	var txHash common.Hash
	err = store.Get(mineDepositKey, &txHash)
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	initialDeposit, _ := new(big.Int).SetString("500000000000000000000", 10)
	if err == storage.ErrNotFound {
		s.logger.Info("no mine deposit tx found, deposit now.")
		err = checkBalance(ctx, swapBackend, erc20Service, overlayEthAddress, s.logger)
		if err != nil {
			return err
		}

		lockupAddress, err := s.contract.Lockup(ctx)
		if err != nil {
			return err
		}

		amount, err := erc20Service.Allowance(ctx, overlayEthAddress, lockupAddress)
		if err != nil {
			return err
		}

		if amount.Cmp(initialDeposit) < 0 {
			hash, err := erc20Service.Approve(ctx, lockupAddress, initialDeposit)
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
		err = store.Put(mineDepositKey, txHash)
		if err != nil {
			return err
		}
	} else {
		s.logger.Infof("waiting for mine deposit in transaction %x", txHash)
	}
	err = s.contract.WaitForDeposit(ctx, txHash)
	if err != nil {
		err = store.Delete(mineDepositKey)
		if err != nil {
			return err
		}
	}
	return err
}

func (s *service) SetTrust(trust Trust) {
	s.trust = trust
}

func (s *service) NotifyCertificate(peer swarm.Address, data []byte) ([]byte, error) {
	// todo check peer is active
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
	trusts, err := s.nodes.TrustAddress(func(a swarm.Address) bool { return !a.Equal(s.base) })
	if err != nil {
		return common.Hash{}, err
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
		signatures, err = s.trust.TrustsSignature(ctx, 1, expire, append(s.base.Bytes(), signature...), trusts...)
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

func checkBalance(
	ctx context.Context,
	swapBackend transaction.Backend,
	erc20Service erc20.Service,
	overlayEthAddress common.Address,
	logger logging.Logger) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, balanceCheckBackoffDuration*time.Duration(balanceCheckMaxRetries))
	defer cancel()

	initialDeposit, _ := new(big.Int).SetString("500000000000000000000", 10)

	for {

		erc20Balance, err := erc20Service.BalanceOf(timeoutCtx, overlayEthAddress)
		if err != nil {
			return err
		}

		ethBalance, err := swapBackend.BalanceAt(timeoutCtx, overlayEthAddress, nil)
		if err != nil {
			return err
		}

		gasPrice, err := swapBackend.SuggestGasPrice(timeoutCtx)
		if err != nil {
			return err
		}

		minimumEth := gasPrice.Mul(gasPrice, big.NewInt(250000))

		insufficientERC20 := erc20Balance.Cmp(initialDeposit) < 0
		insufficientETH := ethBalance.Cmp(minimumEth) < 0

		if insufficientERC20 || insufficientETH {
			neededERC20, mod := new(big.Int).DivMod(initialDeposit, big.NewInt(10000000000000000), new(big.Int))
			if mod.Cmp(big.NewInt(0)) > 0 {
				// always round up the division as the bzzaar cannot handle decimals
				neededERC20.Add(neededERC20, big.NewInt(1))
			}

			if insufficientETH && insufficientERC20 {
				logger.Warningf("cannot continue until there is sufficient ETH (for Gas) and at least %d SANA available on %x", neededERC20, overlayEthAddress)
			} else if insufficientETH {
				logger.Warningf("cannot continue until there is sufficient ETH (for Gas) available on %x", overlayEthAddress)
			} else {
				logger.Warningf("cannot continue until there is at least %d SANA available on %x", neededERC20, overlayEthAddress)
			}
			select {
			case <-time.After(balanceCheckBackoffDuration):
			case <-timeoutCtx.Done():
				if insufficientERC20 {
					return fmt.Errorf("insufficient SANA for initial deposit")
				} else {
					return fmt.Errorf("insufficient ETH for initial deposit")
				}
			}
			continue
		}

		return nil
	}
}
