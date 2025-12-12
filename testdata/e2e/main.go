package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// This is a dummy main package that imports known dependencies
// for e2e testing of license-please.
//
// Expected licenses:
// - github.com/spf13/cobra: Apache-2.0
// - github.com/spf13/pflag: BSD-3-Clause
// - github.com/stretchr/testify: MIT
// - github.com/davecgh/go-spew: ISC (transitive from testify)
// - github.com/pmezard/go-difflib: BSD-3-Clause (transitive from testify)
// - gopkg.in/yaml.v3: MIT/Apache-2.0 (transitive from testify)
// - github.com/inconshreveable/mousetrap: Apache-2.0 (transitive from cobra on windows)

func main() {
	_ = cobra.Command{}
	_ = pflag.FlagSet{}
	_ = assert.True
	fmt.Println("e2e test module")
}
