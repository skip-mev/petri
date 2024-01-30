package cosmosutil

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/skip-mev/petri/types"
)

// GetFeeAmountsFromGasSettings returns the fee amounts from the gas settings
func GetFeeAmountsFromGasSettings(gasSettings types.GasSettings) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(gasSettings.GasDenom, math.NewInt(gasSettings.Gas).Mul(math.NewInt(gasSettings.PricePerGas))))
}
