package core

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
)

// TxChecker implements mempool.TxChecker for CometBFT mempool
type TxChecker struct {
	app abci.Application
}

func NewTxChecker(app abci.Application) *TxChecker {
	return &TxChecker{app: app}
}

// CheckTx implements mempool.TxChecker
func (tc *TxChecker) CheckTx(ctx context.Context, tx []byte) (*abci.ResponseCheckTx, error) {
	resp := tc.app.CheckTx(abci.RequestCheckTx{
		Tx:   tx,
		Type: abci.CheckTxType_New,
	})
	return &resp, nil
}
