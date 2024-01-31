package cosmosutil

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
<<<<<<< HEAD
<<<<<<< HEAD:cosmosutil/util.go
	"github.com/skip-mev/petri/types"
=======
	"github.com/skip-mev/petri/general/v2/types"
>>>>>>> cd1f05b (chore: move everything inside of two packages):cosmos/cosmosutil/util.go
=======
	"github.com/skip-mev/petri/core/v2/types"
>>>>>>> d34ae41 (fix: general -> core)
)

func GetFeeAmountsFromGasSettings(gasSettings types.GasSettings) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(gasSettings.GasDenom, math.NewInt(gasSettings.Gas).Mul(math.NewInt(gasSettings.PricePerGas))))
}
