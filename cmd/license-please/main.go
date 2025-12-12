package main

import (
	"os"

	"github.com/williammartin/licenseplease"
)

func main() {
	licenseplease.Run(os.Args[1:])
}
