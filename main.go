package main

import (
	"github.com/yu-org/yu/apps/asset"
	"github.com/yu-org/yu/apps/poa"
	"github.com/yu-org/yu/core/startup"
)

func main() {
	startup.StartUp(poa.NewPoa(), asset.NewAsset("yu-coin"), NewSidechain())
}
