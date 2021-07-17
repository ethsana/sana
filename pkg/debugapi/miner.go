package debugapi

import (
	"context"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethersphere/bee/pkg/bigint"
	"github.com/ethersphere/bee/pkg/jsonhttp"
)

var (
	errCantPending        = "Cannot get pending"
	errCantWithdrawOfMine = "Cannot withdraw for mine"
)

type withdrawResponse struct {
	Hash common.Hash `json:"hash"`
}

func (s *Service) minerWithdrawHandler(w http.ResponseWriter, r *http.Request) {
	hash, err := s.mine.Withdraw(context.Background())
	if err != nil {
		jsonhttp.InternalServerError(w, errCantWithdrawOfMine)
		return
	}
	jsonhttp.OK(w, withdrawResponse{hash})
}

type pendingResponse struct {
	Balance *bigint.BigInt `json:"balance"`
}

func (s *Service) minerPendingHandler(w http.ResponseWriter, r *http.Request) {
	amount, err := s.mine.Pending(context.Background())
	if err != nil {
		jsonhttp.InternalServerError(w, errCantBalances)
		return
	}

	jsonhttp.OK(w, pendingResponse{Balance: bigint.Wrap(amount)})
}
