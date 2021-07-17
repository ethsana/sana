package config

import (
	"github.com/ethereum/go-ethereum/common"
)

var (
	// chain ID
	ropstenChainID = int64(3)
	goerliChainID  = int64(5)
	xdaiChainID    = int64(100)
	devChainID     = int64(31337)
	// start block
	ropstenStartBlock = uint64(10635691)
	goerliStartBlock  = uint64(4933174)
	xdaiStartBlock    = uint64(16515648)
	devStartBlock     = uint64(0)
	// factory address
	ropstenContractAddress     = common.HexToAddress("0x28598Fd5BEe25E53bC90Cf91B9501F7124f0d5EF")
	goerliContractAddress      = common.HexToAddress("0x0c9de531dcb38b758fe8a2c163444a5e54ee0db2")
	xdaiContractAddress        = common.HexToAddress("0x0FDc5429C50e2a39066D8A94F3e2D2476fcc3b85")
	devContractAddress         = common.HexToAddress("0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512")
	ropstenFactoryAddress      = common.HexToAddress("0xC8dF5a9cdBe9a83d8924e5651e4C448369aAbbf8")
	goerliFactoryAddress       = common.HexToAddress("0x73c412512E1cA0be3b89b77aB3466dA6A1B9d273")
	xdaiFactoryAddress         = common.HexToAddress("0xc2d5a532cf69aa9a1378737d8ccdef884b6e7420")
	goerliLegacyFactoryAddress = common.HexToAddress("0xf0277caffea72734853b834afc9892461ea18474")
	devFactoryAddress          = common.HexToAddress("0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	// postage stamp
	ropstenPostageStampContractAddress = common.HexToAddress("0x2d76025040F254214e6514048DAD44a3277806Ad")
	goerliPostageStampContractAddress  = common.HexToAddress("0x621e455C4a139f5C4e4A8122Ce55Dc21630769E4")
	xdaiPostageStampContractAddress    = common.HexToAddress("0x6a1a21eca3ab28be85c7ba22b2d6eae5907c900e")
	devPostageStampComtractAddress     = common.HexToAddress("0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0")

	// miner address
	ropstenMinerAddress = common.HexToAddress("0xE12763773e5445e3Cc53a3deE93C30A101CcdedC")
	devMinerAddress     = common.HexToAddress("0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9")
)

type ChainConfig struct {
	StartBlock         uint64
	LegacyFactories    []common.Address
	PostageStamp       common.Address
	CurrentFactory     common.Address
	PriceOracleAddress common.Address
	MinerAddress       common.Address
}

func GetChainConfig(chainID int64) (*ChainConfig, bool) {
	var cfg ChainConfig
	switch chainID {
	case ropstenChainID:
		cfg.PostageStamp = ropstenPostageStampContractAddress
		cfg.StartBlock = ropstenStartBlock
		cfg.CurrentFactory = ropstenFactoryAddress
		cfg.LegacyFactories = []common.Address{}
		cfg.PriceOracleAddress = ropstenContractAddress
		cfg.MinerAddress = ropstenMinerAddress
		return &cfg, true

	case goerliChainID:
		cfg.PostageStamp = goerliPostageStampContractAddress
		cfg.StartBlock = goerliStartBlock
		cfg.CurrentFactory = goerliFactoryAddress
		cfg.LegacyFactories = []common.Address{
			goerliLegacyFactoryAddress,
		}
		cfg.PriceOracleAddress = goerliContractAddress
		return &cfg, true
	case xdaiChainID:
		cfg.PostageStamp = xdaiPostageStampContractAddress
		cfg.StartBlock = xdaiStartBlock
		cfg.CurrentFactory = xdaiFactoryAddress
		cfg.LegacyFactories = []common.Address{}
		cfg.PriceOracleAddress = xdaiContractAddress
		return &cfg, true

	case devChainID:
		cfg.PostageStamp = devPostageStampComtractAddress
		cfg.StartBlock = devStartBlock
		cfg.CurrentFactory = devFactoryAddress
		cfg.LegacyFactories = []common.Address{}
		cfg.PriceOracleAddress = devContractAddress
		cfg.MinerAddress = devMinerAddress
		return &cfg, true
	default:
		return &cfg, false
	}
}
