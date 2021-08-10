package oracle

import "math/big"

var defaultPrice = big.NewInt(10000)

type service struct {
}

func New() *service {
	return &service{}
}

func (s *service) Price() *big.Int {
	return new(big.Int).SetBytes(defaultPrice.Bytes())
}
