package sev_test

import (
	"fmt"
	"testing"

	"github.com/ethsana/sana/pkg/mine/tee/sev"
)

func TestOutput(t *testing.T) {
	fmt.Println(sev.Output())
	fmt.Println(sev.Ok())
}
