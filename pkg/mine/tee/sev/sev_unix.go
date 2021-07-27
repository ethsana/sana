// +build !windows

package sev

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var list = []func() *Validate{
	dev_sev,
	sev_enabled_in_kernel,
	dev_sev_readable,
	dev_sev_writable,
	has_kvm_support,
}

type Validate struct {
	Info string
	Pass bool
}

func dev_sev() *Validate {
	info := `Derive: /dev/sev`

	_, err := os.Lstat(`/dev/sev`)
	if err != nil && os.IsNotExist(err) {
		return &Validate{info, false}
	}
	return &Validate{info, true}
}

func sev_enabled_in_kernel() *Validate {
	info := " SEV is enabled in host kernel"

	_, err := os.Lstat("/sys/module/kvm_amd/parameters/sev")
	if err != nil && os.IsNotExist(err) {
		return &Validate{info, false}
	}
	byts, err := ioutil.ReadFile(`/sys/module/kvm_amd/parameters/sev`)
	if err != nil {
		return &Validate{info, false}
	}

	ok := strings.HasPrefix(string(byts), "1")
	return &Validate{info, ok}
}

func dev_sev_readable() *Validate {
	info := ` /dev/sev is readable by user`

	fi, err := os.Lstat(`/dev/sev`)
	if err != nil && os.IsNotExist(err) {
		return &Validate{info, false}
	}
	flag := fi.Mode().Perm() & os.FileMode(256)
	return &Validate{info, flag == os.FileMode(256)}
}

func dev_sev_writable() *Validate {
	info := ` /dev/sev is writable by user`

	fi, err := os.Lstat(`/dev/sev`)
	if err != nil && os.IsNotExist(err) {
		return &Validate{info, false}
	}
	flag := fi.Mode().Perm() & os.FileMode(128)
	return &Validate{info, flag == os.FileMode(128)}
}

func has_kvm_support() *Validate {
	info := `KVM support`
	_, err := os.Lstat(`/dev/kvm`)
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
