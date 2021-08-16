package oracle

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/transaction"
)

var (
	v2PairABI    = transaction.ParseABIUnchecked(UniswapV2PairABI)
	defaultPrice = big.NewInt(10000)
)

type service struct {
	address            common.Address
	backend            transaction.Backend
	transactionService transaction.Service
}

func New(backend transaction.Backend, transactionService transaction.Service, address common.Address) *service {
	return &service{
		address:            address,
		backend:            backend,
		transactionService: transactionService,
	}
}

func (s *service) Price() *big.Int {
	return new(big.Int).SetBytes(defaultPrice.Bytes())
}
