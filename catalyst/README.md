# Catalyst

A load testing tool for Cosmos SDK chains.

## Usage

### As a Library

```go
import "github.com/skip-mev/catalyst/loadtest"

// Create a load test specification
spec := types.LoadTestSpec{
    ChainID:             "my-chain-1",
    BlockGasLimitTarget: 0.8,
    Runtime:             5 * time.Minute,
    NumOfBlocks:         100,
    NodesAddresses:      []types.NodeAddress{...},
    PrivateKeys:         []types.PrivKey{...},
    GasDenom:            "stake",
    Bech32Prefix:        "cosmos",
}

// Create and run the load test
test, err := loadtest.New(ctx, spec)
if err != nil {
    // Handle error
}

result, err := test.Run(ctx)
if err != nil {
    // Handle error
}

fmt.Printf("Total Transactions: %d\n", result.TotalTransactions)
```

### As a Binary

The load test can also be run as a standalone binary using a YAML configuration file.

1. Build the binary:
```bash
go build -o loadtest ./cmd/loadtest
```

2. Create a YAML configuration file (see example in `example/loadtest.yml`):
```yaml
chain_id: "my-chain-1"
block_gas_limit_target: 0.8  # Target 80% of block gas limit
runtime: "5m"  # Run for 5 minutes
num_of_blocks: 100  # Process 100 blocks
nodes_addresses:
  - grpc: "localhost:9090"
    rpc: "http://localhost:26657"
private_keys:
  # Base64-encoded secp256k1 private keys
  - "YOUR_BASE64_ENCODED_PRIVATE_KEY"
gas_denom: "stake"
bech32_prefix: "cosmos"
```

3. Run the load test:
```bash
./loadtest -config path/to/loadtest.yml
```

The binary will execute the load test according to the configuration and print the results to stdout.

#### Private Key Format

The private keys in the YAML configuration should be base64-encoded secp256k1 private keys.

## Results

The load test will output various metrics including:
- Total transactions sent
- Successful transactions
- Failed transactions
- Average gas per transaction
- Average block gas utilization
- Number of blocks processed
- Total runtime
