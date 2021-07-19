package mine

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/crypto"
	"github.com/ethsana/sana/pkg/swarm"
)

func encodePacked(params ...interface{}) []byte {
	byts := make([]byte, 0, len(params)*32)
	for _, v := range params {
		switch val := v.(type) {
		case int32:
		case int64:
			byts = append(byts, common.BigToHash(new(big.Int).SetInt64(int64(val))).Bytes()...)

		case swarm.Address:
			byts = append(byts, val.Bytes()...)

		case common.Hash:
			byts = append(byts, val[:]...)

		case []byte:
			byts = append(byts, val...)
		}

	}
	return byts
}

func recoverSignAddress(signature []byte, params ...interface{}) (common.Address, error) {
	byts := encodePacked(params...)

	hash, err := crypto.LegacyKeccak256(byts)
	if err != nil {
		return common.Address{}, err
	}

	pubKey, err := crypto.Recover(signature, hash)
	if err != nil {
		return common.Address{}, err
	}

	byts, err = crypto.NewEthereumAddress(*pubKey)
	if err != nil {
		return common.Address{}, nil
	}
	return common.BytesToAddress(byts), nil
}

func signLocalTrustData(signer crypto.Signer, params ...interface{}) ([]byte, error) {
	byts := encodePacked(params...)

	hash, err := crypto.LegacyKeccak256(byts)
	if err != nil {
		return nil, err
	}
	return signer.Sign(hash)
}
