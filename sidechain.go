package main

import (
	sidechain "ckb-sidechain-poa/poa"
	"context"
	"encoding/binary"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/nervosnetwork/ckb-sdk-go/address"
	"github.com/nervosnetwork/ckb-sdk-go/crypto/secp256k1"
	"github.com/nervosnetwork/ckb-sdk-go/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/transaction"
	ckbtypes "github.com/nervosnetwork/ckb-sdk-go/types"
	"github.com/nervosnetwork/ckb-sdk-go/utils"
	"github.com/yu-org/yu/apps/poa"
	"github.com/yu-org/yu/common"
	"github.com/yu-org/yu/core/keypair"
	"github.com/yu-org/yu/core/tripod"
	"github.com/yu-org/yu/core/tripod/dev"
	"github.com/yu-org/yu/core/types"
)

type Sidechain struct {
	*tripod.TripodHeader

	ckbCli        rpc.Client
	ckbScript     *ckbtypes.Script
	changeSer     []byte
	lastCkbTxHash []byte

	validators         []keypair.PubKey
	otherValidatorsIps []peer.ID
	myPrivKey          *secp256k1.Secp256k1Key

	N           int
	currentTurn common.BlockNum
}

const MultiSigCode = 99

func NewSidechain(
	ckbUrl string,
	validators []keypair.PubKey,
	myPrivKey keypair.PrivKey,
	otherValidatorsIps []string,
	lastCkbTxHash []byte,
) *Sidechain {
	ckbCli, err := rpc.Dial(ckbUrl)
	if err != nil {
		panic(err)
	}
	ckbPrivKey, err := secp256k1.ToKey(myPrivKey.Bytes())
	if err != nil {
		panic(err)
	}
	pubkeys := make([][]byte, 0)
	for _, validator := range validators {
		pubkeys = append(pubkeys, validator.Bytes())
	}
	changeScript, changeSer, err := address.GenerateSecp256k1MultisigScript(len(validators), len(validators)-1, pubkeys)
	if err != nil {
		panic(err)
	}
	var validatorsIps []peer.ID
	for _, ipStr := range otherValidatorsIps {
		ip, err := peer.Decode(ipStr)
		if err != nil {
			panic(err)
		}
		validatorsIps = append(validatorsIps, ip)
	}

	return &Sidechain{
		TripodHeader:       tripod.NewTripodHeader("ckb-sidechain"),
		ckbCli:             ckbCli,
		ckbScript:          changeScript,
		changeSer:          changeSer,
		validators:         validators,
		myPrivKey:          ckbPrivKey,
		lastCkbTxHash:      lastCkbTxHash,
		otherValidatorsIps: validatorsIps,
		N:                  10,
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
	s.P2pNetwork.SetHandlers(map[int]dev.P2pHandler{
		MultiSigCode: func(msg []byte) ([]byte, error) {
			return s.myPrivKey.Sign(msg)
		},
	})
	return nil
}

func (s *Sidechain) StartBlock(block *types.CompactBlock) error {
	s.currentTurn = block.Height / common.BlockNum(s.N)
	// todo: When last block was sent to Layer1, should check if the block is on chain.
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

	if s.Land.TripodsMap["Poa"].(*poa.Poa).AmILeader(block.Height) {
		return s.sendToLayer1(blocks)
	}
	return nil
}

func (s *Sidechain) FinalizeBlock(block *types.CompactBlock) error {
	return nil
}

func (s *Sidechain) sendToLayer1(blocks []*types.CompactBlock) error {
	evidences := make([]sidechain.Evidence, 0)
	for _, b := range blocks {
		evidences = append(evidences, blockToEvidence(b))
	}
	builder := sidechain.NewEvidencesBuilder()
	es := builder.Set(evidences).Build()
	ckbCost := es.Len()
	esBytes := es.AsSlice()

	systemScript, err := utils.NewSystemScripts(s.ckbCli)
	if err != nil {
		return err
	}
	tx := transaction.NewSecp256k1MultiSigTx(systemScript)
	tx.Outputs = append(tx.Outputs, &ckbtypes.CellOutput{
		Capacity: uint64(ckbCost),
		Lock:     s.ckbScript,
		Type:     nil,
	})
	tx.OutputsData = [][]byte{esBytes}

	var lastTxHash []byte
	if s.currentTurn == 0 {
		lastTxHash = s.lastCkbTxHash
	} else {
		lastTxHash, err = s.State.Get(s, (s.currentTurn - 1).Bytes())
		if err != nil {
			return err
		}
	}

	group, witness, err := transaction.AddInputsForTransaction(tx, []*ckbtypes.CellInput{
		{
			Since: 0,
			PreviousOutput: &ckbtypes.OutPoint{
				TxHash: ckbtypes.BytesToHash(lastTxHash),
				Index:  0,
			},
		},
	}, 65)
	if err != nil {
		return err
	}

	msg, err := transaction.MsgFromTxForMultiSig(tx, group, witness, s.changeSer, len(s.validators))
	if err != nil {
		return err
	}
	sigs, err := s.getSigsFromValidators(msg)
	if err != nil {
		return err
	}

	err = transaction.MultiSignTransaction(tx, group, witness, s.changeSer, sigs...)
	if err != nil {
		return err
	}
	txHash, err := s.ckbCli.SendTransaction(context.Background(), tx)
	if err != nil {
		return err
	}

	s.State.Set(s, s.currentTurn.Bytes(), txHash.Bytes())

	return nil
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

func (s *Sidechain) getSigsFromValidators(msg []byte) ([][]byte, error) {
	sig, err := s.myPrivKey.Sign(msg)
	if err != nil {
		return nil, err
	}

	var sigs = [][]byte{sig}
	for _, ip := range s.otherValidatorsIps {
		respSig, err := s.P2pNetwork.RequestPeer(ip, MultiSigCode, msg)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, respSig)
	}
	return sigs, nil
}
