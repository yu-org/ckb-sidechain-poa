package main

import (
	"github.com/yu-org/yu/apps/asset"
	"github.com/yu-org/yu/apps/poa"
	"github.com/yu-org/yu/common"
	"github.com/yu-org/yu/core/keypair"
	"github.com/yu-org/yu/core/startup"
	"os"
	"strconv"
)

func main() {
	idxStr := os.Args[1]
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		panic(err)
	}

	lastCkbTxHashHex := os.Args[2]

	pubkey, privkey, validators := poa.InitDefaultKeypairs(idx)

	ckbUrl := "http://127.0.0.1:8114"
	lastCkbTxHash := common.FromHex(lastCkbTxHashHex)

	var pubkeys []keypair.PubKey
	var otherIps []string
	for i, validator := range validators {
		pubkeys = append(pubkeys, validator.Pubkey)
		if i != idx {
			otherIps = append(otherIps, validator.P2pIP)
		}
	}

	startup.StartUp(
		poa.NewPoa(pubkey, privkey, validators),
		asset.NewAsset("yu-coin"),
		NewSidechain(ckbUrl, pubkeys, privkey, otherIps, lastCkbTxHash),
	)
}
