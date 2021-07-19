// +build !windows

package sgx

import (
	"bytes"
	"fmt"
	"os"
)

var list = []func() *Validate{
	dev_sgx_enclave,
}

type Validate struct {
	Info string
	Pass bool
}

func dev_sgx_enclave() *Validate {
	info := `Derive /dev/sgx/enclave`

	_, err := os.Lstat(`/dev/sgx/enclave`)
	if err != nil && os.IsNotExist(err) {
		return &Validate{info, false}
	}
	return &Validate{info, true}
}

func Ok() bool {
	for _, valid := range list {
		v := valid()
		if !v.Pass {
			return false
		}
	}
	return true
}

func Output() string {
	buffer := bytes.NewBuffer([]byte{})
	for _, valid := range list {
		v := valid()
		if v.Pass {
			buffer.WriteString(fmt.Sprintf(" \033[0;30;40m%s\033[0m  %s\n", "✔", v.Info))
		} else {
			buffer.WriteString(fmt.Sprintf(" \033[0;31;40m%s\033[0m  %s\n", "✗", v.Info))
		}
	}
	return buffer.String()
}
