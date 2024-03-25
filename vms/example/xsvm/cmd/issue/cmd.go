// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package issue

import (
	"github.com/spf13/cobra"

	"github.com/Juneo-io/juneogo/vms/example/xsvm/cmd/issue/export"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/cmd/issue/importtx"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/cmd/issue/transfer"
)

func Command() *cobra.Command {
	c := &cobra.Command{
		Use:   "issue",
		Short: "Issues transactions",
	}
	c.AddCommand(
		transfer.Command(),
		export.Command(),
		importtx.Command(),
	)
	return c
}
