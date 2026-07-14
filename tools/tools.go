//go:build tools

package tools

import (
	_ "github.com/edaniels/golinters/cmd/combined"
	_ "github.com/polyfloyd/go-errorlint"
	_ "golang.org/x/tools/cmd/goimports"
)
