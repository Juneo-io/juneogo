package txs

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/math"
)

var _ UnsignedTx = (*DonationTx)(nil)

// DonationTx is an unsigned donationTx
type DonationTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// ID of the supernet this tx is modifying
	Supernet ids.ID `serialize:"true" json:"supernetID"`
	// Describes the amount donated
	Amount uint64 `serialize:"true" json:"amount"`
}

func (tx *DonationTx) ConsumedValue(assetID ids.ID) uint64 {
	value := tx.BaseTx.ConsumedValue(assetID)
	val, err := math.Sub(value, tx.Amount)
	if err != nil {
		return uint64(0)
	}
	return val
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [DonationTx]. Also sets the [ctx] to the given [vm.ctx] so
// that the addresses can be json marshalled into human readable format
func (tx *DonationTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
}

// SyntacticVerify returns nil iff [tx] is valid
func (tx *DonationTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *DonationTx) Visit(visitor Visitor) error {
	return visitor.DonationTx(tx)
}
