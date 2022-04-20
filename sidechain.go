package main

import (
	sidechain "ckb-sidechain-poa/poa"
	"encoding/binary"
	"github.com/yu-org/yu/apps/poa"
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
	return nil
}

func (s *Sidechain) StartBlock(block *types.CompactBlock) error {
	return nil
}

func (s *Sidechain) EndBlock(block *types.CompactBlock) error {
	height := block.Height
	n := common.BlockNum(s.N)

	// Send blocks to Layer1 Block Height:  0~(n-1), n~(2n-1), 2n~(3n-1), ...
	if height%n != n-1 {
		return nil
	}
	blocks, err := s.Chain.GetRangeBlocks(height+1-n, height)
	if err != nil {
		return err
	}

	evidences := make([]sidechain.Evidence, 0)
	for _, b := range blocks {
		evidences = append(evidences, blockToEvidence(b))
	}

	if s.Land.TripodsMap["Poa"].(*poa.Poa).AmILeader(block.Height) {
		return s.sendToLayer1(evidences)
	}
	return nil
}

func (s *Sidechain) FinalizeBlock(block *types.CompactBlock) error {
	return nil
}

func (s *Sidechain) sendToLayer1(e []sidechain.Evidence) error {
	builder := sidechain.NewEvidencesBuilder()
	es := builder.Set(e).Build()
	esBytes := es.AsSlice()

}

func blockToEvidence(block *types.CompactBlock) sidechain.Evidence {
	builder := sidechain.NewEvidenceBuilder()
	// block hash
	blockHash := *sidechain.HashFromSliceUnchecked(block.Hash.Bytes())
	builder.BlockHash(blockHash)
	// block height
	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(heightBytes, uint32(block.Height))
	height := *sidechain.Uint64FromSliceUnchecked(heightBytes)
	builder.Height(height)
	// state root
	stateRoot := *sidechain.HashFromSliceUnchecked(block.StateRoot.Bytes())
	builder.StateRoot(stateRoot)
	// txn root
	txnRoot := *sidechain.HashFromSliceUnchecked(block.TxnRoot.Bytes())
	builder.TxnRoot(txnRoot)

	return builder.Build()
}
