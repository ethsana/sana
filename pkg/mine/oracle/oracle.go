package oracle

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethsana/sana/pkg/logging"
	"github.com/ethsana/sana/pkg/transaction"
)

var (
	defaultPrice   = big.NewInt(10000)
	v2PairABI      = transaction.ParseABIUnchecked(UniswapV2PairABI)
	decimalSANA    = new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil)
	decimalETH     = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	v2PairETH_USDT = common.HexToAddress("0x0d4a11d5eeaac28ec3f61d100daf4d40471f1852")
)

var (
	errDecodeABI = errors.New("could not decode abi data")
)

type service struct {
	address   common.Address
	backend   transaction.Backend
	logger    logging.Logger
	validTime time.Duration

	price    *big.Int
	expire   time.Time
	priceMtx sync.RWMutex
}

func New(
	endpoint string,
	v2PairAddress common.Address,
	validTime time.Duration,
	logger logging.Logger,
) (*service, error) {
	backend, err := ethclient.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("dial eth client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chainID, err := backend.ChainID(ctx)
	if err != nil {
		logger.Infof("could not connect to backend at %v. In a uniswap-enabled network a working blockchain node  is required. Check your node or specify another node using --uniswap-endpoint.", endpoint)
		return nil, fmt.Errorf("get chain id: %w", err)
	}

	if chainID.Cmp(big.NewInt(1)) != 0 {
		logger.Infof("The current endpoint is not the ETH mainnet.")
		return nil, fmt.Errorf("illegal chain id")
	}
	return &service{
		address:   v2PairAddress,
		backend:   backend,
		logger:    logger,
		validTime: validTime,
	}, nil
}

func (s *service) Price(ctx context.Context) (*big.Int, error) {
	return defaultPrice, nil
}

func (s *service) PriceNew(ctx context.Context) (_ *big.Int, err error) {
	now := time.Now()
	s.priceMtx.Lock()
	defer s.priceMtx.Unlock()
	if s.expire.Before(now) {
		s.logger.Tracef("price with Uniswap expired")
		s.price, err = s.updatePrice(ctx)
		if err != nil {
			return nil, err
		}
		s.expire = time.Now().Add(s.validTime)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	default:
	}

	return s.price, nil
}

func (s *service) getReserves(ctx context.Context, address common.Address) (reserve1 *big.Int, reserve2 *big.Int, err error) {
	callData, err := v2PairABI.Pack(`getReserves`)
	if err != nil {
		return nil, nil, err
	}
	msg := ethereum.CallMsg{
		From: common.Address{},
		To:   &address,
		Data: callData,
	}
	output, err := s.backend.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, nil, err
	}

	results, err := v2PairABI.Unpack(`getReserves`, output)
	if err != nil {
		return nil, nil, err
	}

	if len(results) != 3 {
		return nil, nil, errDecodeABI
	}
	reserve1, ok := abi.ConvertType(results[0], new(big.Int)).(*big.Int)
	if !ok {
		return nil, nil, errDecodeABI
	}
	reserve2, ok = abi.ConvertType(results[1], new(big.Int)).(*big.Int)
	if !ok {
		return nil, nil, errDecodeABI
	}
	return reserve1, reserve2, nil
}

func (s *service) getPriceWithETH(ctx context.Context) (*big.Int, error) {
	reserve1, reserve2, err := s.getReserves(ctx, v2PairETH_USDT)
	if err != nil {
		return nil, err
	}

	return new(big.Int).Div(new(big.Int).Mul(reserve2, decimalETH), reserve1), nil
}

func (s *service) updatePrice(ctx context.Context) (*big.Int, error) {
	ethPrice, err := s.getPriceWithETH(ctx)
	if err != nil {
		return nil, err
	}
	reserve1, reserve2, err := s.getReserves(ctx, s.address)
	if err != nil {
		return nil, err
	}
	priceWithETH := new(big.Int).Div(new(big.Int).Mul(reserve2, decimalSANA), reserve1)

	return new(big.Int).Div(new(big.Int).Mul(ethPrice, priceWithETH), decimalETH), nil
}
