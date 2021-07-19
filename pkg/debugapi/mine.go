package debugapi

import (
	"context"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/bigint"
	"github.com/ethsana/sana/pkg/jsonhttp"
)

var (
	errMineDisable           = "mine disable"
	errCantMineStatus        = "Cannot get mine status"
	errCantWithdrawOfMine    = "Cannot withdraw for mine"
	errCantCashDepositOfMine = "Cannot cashdeposit for mine"
)

type withdrawResponse struct {
	Hash common.Hash `json:"hash"`
}

func (s *Service) mineWithdrawHandler(w http.ResponseWriter, r *http.Request) {
	if !s.minerEnabled {
		jsonhttp.InternalServerError(w, errMineDisable)
		return
	}

	hash, err := s.mine.Withdraw(context.Background())
	if err != nil {
		jsonhttp.InternalServerError(w, errCantWithdrawOfMine)
		return
	}
	jsonhttp.OK(w, withdrawResponse{hash})
}

func (s *Service) mineCashDepositHandler(w http.ResponseWriter, r *http.Request) {
	if !s.minerEnabled {
		jsonhttp.InternalServerError(w, errMineDisable)
		return
	}

	hash, err := s.mine.CashDeposit(context.Background())
	if err != nil {
		jsonhttp.InternalServerError(w, errCantCashDepositOfMine)
		return
	}
	jsonhttp.OK(w, withdrawResponse{hash})
}

type mineStatusResponse struct {
	Work    bool           `json:"work"`
	Reward  *bigint.BigInt `json:"reward"`
	Pending *bigint.BigInt `json:"pending"`
	Expire  *bigint.BigInt `json:"expire"`
}

func (s *Service) mineStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !s.minerEnabled {
		jsonhttp.InternalServerError(w, errMineDisable)
		return
	}

	work, reward, pending, expire, err := s.mine.Status(context.Background())
	if err != nil {
		s.logger.Infof("get mine status err: %s", err)
		jsonhttp.InternalServerError(w, errCantMineStatus)
		return
	}

	jsonhttp.OK(w, mineStatusResponse{
		Work:    work,
		Reward:  bigint.Wrap(reward),
		Pending: bigint.Wrap(pending),
		Expire:  bigint.Wrap(expire),
	})
}
