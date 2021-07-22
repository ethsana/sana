// Copyright 2021 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"runtime"

	"github.com/ethsana/sana/pkg/mine/tee/sev"
	"github.com/ethsana/sana/pkg/mine/tee/sgx"

	"github.com/spf13/cobra"
)

func (c *command) initTeeCmd() {
	v := &cobra.Command{
		Use:   "tee",
		Short: "Print TEE related information",
		Run: func(cmd *cobra.Command, args []string) {
			if runtime.GOOS == `windows` {
				cmd.Println(`Currently does not support the current system`)
				return
			}
			cmd.Println(`Platform: AMD`)
			cmd.Print(sev.Output())

			cmd.Println(`Platform: Intel`)
			cmd.Print(sgx.Output())
		},
	}
	v.SetOut(c.root.OutOrStdout())
	c.root.AddCommand(v)
}
