package debugapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/bigint"
	"github.com/ethsana/sana/pkg/jsonhttp"
)

var errMineDisable = "Mine disable"

type withdrawResponse struct {
	Hash common.Hash `json:"hash"`
}

func (s *Service) mineWithdrawHandler(w http.ResponseWriter, r *http.Request) {
	if !s.minerEnabled {
		jsonhttp.InternalServerError(w, errMineDisable)
		return
	}

	ctx, cancal := context.WithTimeout(context.Background(), time.Second*10)
	defer cancal()

	hash, err := s.mine.Withdraw(ctx)
	if err != nil {
		jsonhttp.InternalServerError(w, fmt.Sprint("Cannot withdraw for mine at: ", err.Error()))
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
		jsonhttp.InternalServerError(w, fmt.Sprint("Cannot cashdeposit for mine at:", err.Error()))
		return
	}
	jsonhttp.OK(w, withdrawResponse{hash})
}

type mineStatusResponse struct {
	Work    bool           `json:"work"`
	Reward  *bigint.BigInt `json:"reward"`
	Pending *bigint.BigInt `json:"pending"`
	Expire  *bigint.BigInt `json:"expire"`
	Deposit *bigint.BigInt `json:"deposit"`
}

func (s *Service) mineStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !s.minerEnabled {
		jsonhttp.InternalServerError(w, errMineDisable)
		return
	}

	work, reward, pending, expire, deposit, err := s.mine.Status(context.Background())
	if err != nil {
		s.logger.Infof("get mine status err: %s", err)
		jsonhttp.InternalServerError(w, fmt.Sprint("Cannot get mine status at:", err.Error()))
		return
	}

	jsonhttp.OK(w, mineStatusResponse{
		Work:    work,
		Reward:  bigint.Wrap(reward),
		Pending: bigint.Wrap(pending),
		Expire:  bigint.Wrap(expire),
		Deposit: bigint.Wrap(deposit),
	})
}
