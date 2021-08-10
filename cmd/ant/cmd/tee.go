// Copyright 2021 The Sana Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	tee "github.com/ethsana/sana-tee"
	"github.com/spf13/cobra"
)

func (c *command) initTeeCmd() {
	v := &cobra.Command{
		Use:   "tee",
		Short: "Print TEE related information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(tee.Output())
		},
	}
	v.SetOut(c.root.OutOrStdout())
	c.root.AddCommand(v)
}
