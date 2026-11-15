// Command harness is the CSI/storage certification test harness CLI.
//
// Tool integrations are registered by blank-importing their packages in root.go —
// that single import line is all it takes to add a compiled-in tool. See
// decisions/0003.
package main

import (
	"log/slog"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}
