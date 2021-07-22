package nodeservice_test

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestZeroAddress(t *testing.T) {
	t.Log(common.Address{}.String())
	t.Log(common.Address{} == common.Address{})
}

func TestDefer(t *testing.T) {
	defer func() { fmt.Println(2222) }()
	defer func() { fmt.Println(111) }()
}
