package sgx_test

import (
	"fmt"
	"testing"

	"github.com/ethsana/sana/pkg/mine/tee/sgx"
)

func TestOutput(t *testing.T) {
	fmt.Println(sgx.Output())
	fmt.Println(sgx.Ok())
}
