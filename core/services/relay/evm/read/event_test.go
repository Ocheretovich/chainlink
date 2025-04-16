package read

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/smartcontractkit/chainlink-ccip/chains/evm/gobindings/generated/latest/offramp"
	"github.com/smartcontractkit/chainlink-ccip/pkg/reader"
	"github.com/smartcontractkit/chainlink-evm/pkg/logpoller"

	"github.com/stretchr/testify/require"
)

func Test_DecodeHardcodedType(t *testing.T) {
	t.Parallel()

	fixtLog := getFixtureLog()

	log, err := generateOfframpLog(fixtLog)
	require.NoError(t, err)

	var out reader.CommitReportAcceptedEvent

	t.Run("decode hardcoded type offramp success", func(t *testing.T) {
		err = decodeHardcodedType(&out, log)
		require.NoError(t, err)

		require.Equal(t, true, bytes.Equal(fixtLog.BlessedMerkleRoots[0].MerkleRoot[:], out.BlessedMerkleRoots[0].MerkleRoot[:]))
		require.Equal(t, true, bytes.Equal(fixtLog.UnblessedMerkleRoots[0].MerkleRoot[:], out.UnblessedMerkleRoots[0].MerkleRoot[:]))
	})
}

func generateOfframpLog(log offramp.OffRampCommitReportAccepted) (*logpoller.Log, error) {
	event := offrampABI.Events[commitReportAcceptedEvent]
	data, err := event.Inputs.NonIndexed().Pack(log.BlessedMerkleRoots, log.UnblessedMerkleRoots, log.PriceUpdates)
	if err != nil {
		return nil, err
	}

	res := &logpoller.Log{}
	res.Data = data
	res.Topics = append(res.Topics, event.ID.Bytes())

	return res, nil
}

func getFixtureLog() offramp.OffRampCommitReportAccepted {
	var res offramp.OffRampCommitReportAccepted
	res.BlessedMerkleRoots = []offramp.InternalMerkleRoot{
		{
			SourceChainSelector: 1234,
			OnRampAddress:       bytes.Repeat([]byte{0x11}, 20),
			MinSeqNr:            1,
			MaxSeqNr:            10,
			MerkleRoot:          [32]byte{0xaa},
		},
	}
	res.UnblessedMerkleRoots = []offramp.InternalMerkleRoot{
		{
			SourceChainSelector: 1234,
			OnRampAddress:       bytes.Repeat([]byte{0x11}, 20),
			MinSeqNr:            1,
			MaxSeqNr:            10,
			MerkleRoot:          [32]byte{0xab},
		},
	}
	res.PriceUpdates.TokenPriceUpdates = []offramp.InternalTokenPriceUpdate{

		{
			SourceToken: common.HexToAddress("0x2222222222222222222222222222222222222222"),
			UsdPerToken: big.NewInt(1e18),
		},
	}
	res.PriceUpdates.GasPriceUpdates = []offramp.InternalGasPriceUpdate{
		{
			DestChainSelector: 5678,
			UsdPerUnitGas:     big.NewInt(2e18),
		},
	}

	return res
}
