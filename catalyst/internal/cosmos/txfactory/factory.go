package txfactory

import (
	"fmt"
	"math/rand"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/skip-mev/catalyst/internal/cosmos/wallet"
	"github.com/skip-mev/catalyst/internal/types"
)

// TxFactory creates transactions for load testing
type TxFactory struct {
	gasDenom string
	wallets  []*wallet.InteractingWallet
}

// NewTxFactory creates a new transaction factory
func NewTxFactory(gasDenom string, wallets []*wallet.InteractingWallet) *TxFactory {
	return &TxFactory{
		gasDenom: gasDenom,
		wallets:  wallets,
	}
}

// CreateMsg creates a message of the specified type
func (f *TxFactory) CreateMsg(msgType types.MsgType, fromWallet *wallet.InteractingWallet) (sdk.Msg, error) {
	switch msgType {
	case types.MsgSend:
		return f.createMsgSend(fromWallet)
	case types.MultiMsgSend:
		return f.createMsgMultiSend(fromWallet)
	default:
		return nil, fmt.Errorf("unsupported message type: %v", msgType)
	}
}

// createMsgSend creates a basic bank send message
func (f *TxFactory) createMsgSend(fromWallet *wallet.InteractingWallet) (sdk.Msg, error) {
	amount := sdk.NewCoins(sdk.NewCoin(f.gasDenom, sdkmath.NewInt(1000000)))

	var toWallet *wallet.InteractingWallet
	if len(f.wallets) == 1 {
		toWallet = fromWallet
	} else {
		// Keep selecting until we get a different wallet
		for {
			toWallet = f.wallets[rand.Intn(len(f.wallets))]
			if toWallet.FormattedAddress() != fromWallet.FormattedAddress() {
				break
			}
		}
	}

	fromAddr, err := sdk.AccAddressFromBech32(fromWallet.FormattedAddress())
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}

	toAddr, err := sdk.AccAddressFromBech32(toWallet.FormattedAddress())
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	return banktypes.NewMsgSend(fromAddr, toAddr, amount), nil
}

// createMsgMultiSend creates a multi-send message that distributes funds to all other wallets
func (f *TxFactory) createMsgMultiSend(fromWallet *wallet.InteractingWallet) (sdk.Msg, error) {
	numRecipients := len(f.wallets) - 1
	if numRecipients == 0 {
		numRecipients = 1
	}
	amountPerRecipient := sdk.NewCoins(sdk.NewCoin(f.gasDenom, sdkmath.NewInt(1000000/int64(numRecipients))))

	// Create outputs for all other wallets
	outputs := make([]banktypes.Output, 0, numRecipients)
	totalAmount := sdk.NewCoins()
	for _, w := range f.wallets {
		if w.FormattedAddress() == fromWallet.FormattedAddress() {
			continue // skip sender
		}
		outputs = append(outputs, banktypes.Output{
			Address: w.FormattedAddress(),
			Coins:   amountPerRecipient,
		})
		totalAmount = totalAmount.Add(amountPerRecipient...)
	}

	// If no other wallets, send back to self
	if len(outputs) == 0 {
		outputs = append(outputs, banktypes.Output{
			Address: fromWallet.FormattedAddress(),
			Coins:   amountPerRecipient,
		})
		totalAmount = amountPerRecipient
	}

	return &banktypes.MsgMultiSend{
		Inputs: []banktypes.Input{
			{
				Address: fromWallet.FormattedAddress(),
				Coins:   totalAmount,
			},
		},
		Outputs: outputs,
	}, nil
}
