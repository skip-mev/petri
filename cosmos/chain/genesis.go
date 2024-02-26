package chain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/icza/dyno"
)

// GenesisKV is used in ModifyGenesis to specify which keys have to be modified
// in the resulting genesis

type GenesisKV struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// GenesisModifier represents a type of function that takes in genesis formatted in bytes
// and returns a modified genesis file in the same format

type GenesisModifier func([]byte) ([]byte, error)

var _ GenesisModifier = ModifyGenesis(nil)

// NewGenesisKV is a function for creating a GenesisKV object
func NewGenesisKV(key string, value interface{}) GenesisKV {
	return GenesisKV{
		Key:   key,
		Value: value,
	}
}

// ModifyGenesis is a function that is a GenesisModifier and takes in GenesisKV
// to specify which fields of the genesis file should be modified
func ModifyGenesis(genesisKV []GenesisKV) func([]byte) ([]byte, error) {
	return func(genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}

		for idx, values := range genesisKV {
			splitPath := strings.Split(values.Key, ".")

			path := make([]interface{}, len(splitPath))
			for i, component := range splitPath {
				if v, err := strconv.Atoi(component); err == nil {
					path[i] = v
				} else {
					path[i] = component
				}
			}

			if err := dyno.Set(g, values.Value, path...); err != nil {
				return nil, fmt.Errorf("failed to set value (index:%d) in genesis json: %w", idx, err)
			}
		}

		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
