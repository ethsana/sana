package config

import (
	"github.com/ethereum/go-ethereum/common"
)

var (
	// chain ID
	goerliChainID = int64(5)
	xdaiChainID   = int64(100)
	devChainID    = int64(31337)
	// start block
	goerliStartBlock = uint64(5167986)
	xdaiStartBlock   = uint64(17525957)
	devStartBlock    = uint64(0)
	// factory address
	goerliContractAddress = common.HexToAddress("0x2469391F81F38313CfC6BfBb6132EDf27B0d55A0")
	xdaiContractAddress   = common.HexToAddress("0x61E7cdF724446aAfCBF56312b501a0072Ae90Eee")
	devContractAddress    = common.HexToAddress("0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512")
	goerliFactoryAddress  = common.HexToAddress("0x6737bA0bFA19EEDf1FC7FA73947bc5885F4b511c")
	xdaiFactoryAddress    = common.HexToAddress("0xf1829378C26cE9f8D3b1b8610334258515d0A7B5")
	// goerliLegacyFactoryAddress = common.HexToAddress("0xf0277caffea72734853b834afc9892461ea18474")
	devFactoryAddress = common.HexToAddress("0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	// postage stamp
	goerliPostageStampContractAddress = common.HexToAddress("0x2D1Fb33057a5022a870707aEe74eA991fDed764e")
	xdaiPostageStampContractAddress   = common.HexToAddress("0xeb2baF84d972a091232654CaCc719E6D833E2118")
	devPostageStampComtractAddress    = common.HexToAddress("0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0")

	// miner address
	goerliMinerAddress = common.HexToAddress("0xfeb4c0E59329A183c75bbE579c3aC4915241Af0c")
	devMinerAddress    = common.HexToAddress("0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9")
	xdaiMinerAddress   = common.HexToAddress("0x36032EA08fbdE8143e53afa9C752AcD87e8FAd7C")

	// uniswap v2 pool
	// devUniV2Pair =
	mainUniV2Pair = common.HexToAddress("0xa67741e5929d970c133cc8943ce3b8d9115bf392")
)

type ChainConfig struct {
	StartBlock         uint64
	LegacyFactories    []common.Address
	PostageStamp       common.Address
	CurrentFactory     common.Address
	PriceOracleAddress common.Address
	MinerAddress       common.Address
	UniV2PairAddress   common.Address
}

func GetChainConfig(chainID int64) (*ChainConfig, bool) {
	var cfg ChainConfig
	switch chainID {
	case goerliChainID:
		cfg.PostageStamp = goerliPostageStampContractAddress
		cfg.StartBlock = goerliStartBlock
		cfg.CurrentFactory = goerliFactoryAddress
		cfg.LegacyFactories = []common.Address{}
		cfg.PriceOracleAddress = goerliContractAddress
		cfg.MinerAddress = goerliMinerAddress
		cfg.UniV2PairAddress = mainUniV2Pair
		return &cfg, true

	case xdaiChainID:
		cfg.PostageStamp = xdaiPostageStampContractAddress
		cfg.StartBlock = xdaiStartBlock
		cfg.CurrentFactory = xdaiFactoryAddress
		cfg.LegacyFactories = []common.Address{}
		cfg.PriceOracleAddress = xdaiContractAddress
		cfg.MinerAddress = xdaiMinerAddress
		cfg.UniV2PairAddress = mainUniV2Pair
		return &cfg, true

	case devChainID:
		cfg.PostageStamp = devPostageStampComtractAddress
		cfg.StartBlock = devStartBlock
		cfg.CurrentFactory = devFactoryAddress
		cfg.LegacyFactories = []common.Address{}
		cfg.PriceOracleAddress = devContractAddress
		cfg.MinerAddress = devMinerAddress
		cfg.UniV2PairAddress = mainUniV2Pair
		return &cfg, true

	default:
		return &cfg, false
	}
}
