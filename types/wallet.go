package types

import cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"

type WalletI interface {
	FormattedAddress() string
	KeyName() string
	Address() []byte
	FormattedAddressWithPrefix(prefix string) string
	PublicKey() (cryptotypes.PubKey, error)
	PrivateKey() (cryptotypes.PrivKey, error)
}
