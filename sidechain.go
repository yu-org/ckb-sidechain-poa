package main

import (
	"ckb-sidechain-poa/poa"
	"encoding/binary"
	"github.com/yu-org/yu/common"
	"github.com/yu-org/yu/core/tripod"
	"github.com/yu-org/yu/core/types"
)

type Sidechain struct {
	*tripod.TripodHeader

	N int
}

func NewSidechain() *Sidechain {

	return &Sidechain{
		TripodHeader: tripod.NewTripodHeader("ckb-sidechain"),
		N:            10,
	}
}

func (s *Sidechain) GetTripodHeader() *tripod.TripodHeader {
	return s.TripodHeader
}

func (s *Sidechain) CheckTxn(txn *types.SignedTxn) error {
	return nil
}

func (s *Sidechain) VerifyBlock(block *types.CompactBlock) bool {
	return true
}

func (s *Sidechain) InitChain() error {
	genesisBlock, err := s.Chain.GetGenesis()
	if err != nil {
		return err
	}

}

func (s *Sidechain) StartBlock(block *types.CompactBlock) error {
	return nil
}

func (s *Sidechain) EndBlock(block *types.CompactBlock) error {
	height := block.Height
	n := common.BlockNum(s.N)
	if height % n != 0 {
		return nil
	}
	blocks, err := s.Chain.GetRangeBlocks(height - n + 1, height)
	if err != nil {
		return err
	}

	evidences := make([]poa.Evidence, 0)
	for _, b := range blocks {
		evidences = append(evidences, blockToEvidence(b))
	}

	return s.sendToLayer1(evidences)
}

func (s *Sidechain) FinalizeBlock(block *types.CompactBlock) error {
	return nil
}

func (s *Sidechain) sendToLayer1(e []poa.Evidence) error {
	builder := poa.NewEvidencesBuilder()
	es := builder.Set(e).Build()
	esBytes := es.AsSlice()

}

func blockToEvidence(block *types.CompactBlock) poa.Evidence {
	builder := poa.NewEvidenceBuilder()

	blockHash := *poa.HashFromSliceUnchecked(block.Hash.Bytes())
	builder.BlockHash(blockHash)

	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(heightBytes, uint32(block.Height))
	height := *poa.Uint64FromSliceUnchecked(heightBytes)
	builder.Height(height)

	stateRoot := *poa.HashFromSliceUnchecked(block.StateRoot.Bytes())
	builder.StateRoot(stateRoot)

	txnRoot := *poa.HashFromSliceUnchecked(block.TxnRoot.Bytes())
	builder.TxnRoot(txnRoot)


	return builder.Build()
}