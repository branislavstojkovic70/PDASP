# PDASP: trading system on Hyperledger Fabric

Course project for PDASP 2024/25, Faculty of Technical Sciences.

A trading system for arbitrary goods running on a Hyperledger Fabric network, with
a Node.js console client that talks to the chaincode through the SDK. The original
assignment is in [specifikacija-projekta.md](specifikacija-projekta.md).

```
 3 organizations x 3 peers  |  3 RAFT orderers  |  4 Fabric CAs  |  9 CouchDB
 2 channels, every peer on both  |  chaincode in Go  |  client in Node.js
```

---

## 1. What it does

- **Merchant types** are a catalogue kept on the ledger, not an enum in code.
- **Merchants** offer products and collect money; **customers** buy and pay.
- **Buying** checks stock and funds, moves money both ways, issues an **invoice**
  to both parties, and deletes the product once it sells out.
- **Searching** products by name, code, merchant type and price, individually or
  in any combination, using CouchDB rich queries.
- The console client **enrols identities** against Fabric CA and works with
  certificates from all three organizations.

---

## 2. Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Docker | 20.10+ | Docker Desktop on macOS or Windows, the daemon must be running |
| Go | 1.23+ | for building and testing the chaincode |
| Node.js | 20+ | for the console client |
| jq | any | used by the network scripts and the tests |
| bash | 3.2+ | the stock macOS bash is enough |

The Fabric binaries and docker images are not prerequisites; the install script
downloads them.

```bash
brew install go node jq        # macOS
./network/install-fabric.sh    # Fabric 2.5.16 binaries + all docker images
```

---

## 3. Quick start

```bash
./network/install-fabric.sh        # once: binaries and docker images
./network/network-up.sh            # CAs, crypto material, genesis blocks, nodes
./network/create-channels.sh       # channel1 and channel2, join all 9 peers
./network/deploy-chaincode.sh      # package, install, approve, commit, InitLedger

cd application && npm install
node src/index.js bootstrap        # enrol the standard identities
node src/index.js                  # interactive menu
```

Then, from the repository root:

```bash
./test/run-all-tests.sh            # 10 scripts, 117 assertions
```

Tearing everything down:

```bash
./network/network-down.sh          # stops the network and deletes what was generated
```

A full rebuild from nothing takes about 25 seconds for the network, plus roughly a
minute for the chaincode deployment.

---

## 4. Layout

```
network/       Fabric network: compose files, configtx, scripts
chaincode/     the `trade` chaincode, in Go
application/   the Node.js console client
test/          shell test scripts
```

---

## 5. The network

Three peer organizations, mapped onto the business domain as follows. The mapping
is organizational, not enforced in the chaincode, which the assignment explicitly
permits.

| Org | MSP | Role | Peers | CA |
|---|---|---|---|---|
| Org1 | Org1MSP | Merchants | peer0/1/2.org1.pdasp.com | localhost:7054 |
| Org2 | Org2MSP | Customers | peer0/1/2.org2.pdasp.com | localhost:8054 |
| Org3 | Org3MSP | Regulator | peer0/1/2.org3.pdasp.com | localhost:9054 |

Ordering is a three node RAFT cluster (`orderer`, `orderer2`, `orderer3`), quorum
2 of 3. Both `channel1` and `channel2` carry all nine peers and the same chaincode
but keep entirely separate ledgers.

Two settings the assignment asks for by name:

- **Block cutting**: `MaxMessageCount: 2`, `BatchTimeout: 1s`. A block is cut after
  two transactions or one second, whichever comes first.
- **Endorsement policy**: `OutOf(2, 'Org1MSP.peer', 'Org2MSP.peer', 'Org3MSP.peer')`,
  a majority.

Useful endpoints while the network is up:

```
CouchDB Fauxton (peer0.org1)   http://localhost:5984/_utils   admin / adminpw
Peer operations (peer0.org1)   http://localhost:9451/healthz
Orderer operations             http://localhost:9440/healthz
```

---

## 6. The console client

Two modes. Without arguments it opens an interactive menu; with a command it runs
that one command and exits, which is what the shell tests use.

```bash
cd application
node src/index.js                  # interactive menu
node src/index.js help             # every command
node src/index.js help buy         # one command
```

Flags shared by every command that touches the ledger:

| Flag | Default | Meaning |
|---|---|---|
| `--org` | `org1` | organization whose identity signs |
| `--user` | `org1user1` | wallet identity |
| `--channel` | `channel1` | `channel1` or `channel2` |
| `--peer` | `0` | which peer of the organization to dial |
| `--compact` | off | print the JSON result on one line |

### Identities

`network/scripts/register-enroll.sh` registers `admin`, `<org>admin` and
`<org>user1` for each organization with fixed secrets, so those enrol directly.
Anything else has to be registered first.

```bash
node src/index.js bootstrap                              # enrol all nine standard identities
node src/index.js enroll --org org2 --user org2user1     # enrol one
node src/index.js enroll --org org1 --user clerk --register   # register a new one, then enrol
node src/index.js identities
node src/index.js whoami --org org2 --user org2user1
```

### Working with the ledger

```bash
# Merchants and products, as an Org1 identity
node src/index.js create-merchant M010 "Corner Shop" SUPERMARKET 123456789 5000 --org org1 --user org1admin
node src/index.js add-product M010 P100 "Rye bread 700g" 95.50 40 2026-12-01 --org org1 --user org1admin

# Customers and buying, as an Org2 identity
node src/index.js create-customer C010 Mira Miric mira@example.com 8000 --org org2 --user org2user1
node src/index.js buy C010 M010 P100 3 --org org2 --user org2user1
node src/index.js deposit customer C010 2500 --org org2 --user org2user1

# Searching, as the Org3 read only identity
node src/index.js search-name milk --org org3 --user org3user1
node src/index.js search-type PHARMACY --org org3 --user org3user1
node src/index.js search-price 100 400 --org org3 --user org3user1
node src/index.js search --name milk --type SUPERMARKET,PHARMACY --price-from 1000 --price-to 2000 --sort asc
node src/index.js expiring 2026-10-01
node src/index.js search-paged --page-size 5

# Invoices
node src/index.js invoices --customer C010
node src/index.js invoices --merchant M010
node src/index.js invoices --customer C010 --min 250
```

Results are printed as JSON on stdout and progress notes on stderr, so piping into
`jq` works:

```bash
node src/index.js products --compact | jq '[.[] | {code, name, price}]'
```

### Bulk input

The bulk commands take a JSON array from a file:

```bash
cat > /tmp/products.json <<'JSON'
[
  {"code":"P200","name":"Olive oil 1L","price":1290,"quantity":15,"expiryDate":"2028-02-01"},
  {"code":"P201","name":"Sea salt 500g","price":89,"quantity":60}
]
JSON
node src/index.js add-products M010 --file /tmp/products.json --org org1 --user org1admin
```

---

## 7. Testing

Two layers.

**Chaincode unit tests** run against an in-memory ledger with a simplified CouchDB
Mango evaluator, so the query selectors themselves are under test:

```bash
cd chaincode/trade && go test ./... -cover
```

**End to end shell tests** drive the console client against the running network,
which is what the assignment requires:

```bash
./test/run-all-tests.sh          # everything
./test/run-all-tests.sh 05 08    # only the purchase and error handling scripts
```

| Script | Covers |
|---|---|
| `test-01-enroll.sh` | enrolment, three MSPs, registering a new identity |
| `test-02-merchants.sh` | merchant type catalogue, merchants, bulk, type propagation |
| `test-03-customers.sh` | customers, single and bulk, duplicate rejection |
| `test-04-products.sh` | products, bulk, price, restock |
| `test-05-purchase.sh` | totals, balances, invoices, sold out product removal |
| `test-06-deposit.sh` | deposits and money rounding |
| `test-07-search.sh` | every search, combined selector, sorting, pagination |
| `test-08-errors.sh` | missing records, insufficient funds and stock, invalid input |
| `test-09-two-certificates.sh` | identities from three organizations, all peers |
| `test-10-two-channels.sh` | the two channels holding separate ledgers |

The suite is repeatable: records are created with ids suffixed by a run id taken
from the clock, and assertions are relative wherever the ledger accumulates.

---

## 8. Chaincode API

Invoked as `node src/index.js <command>`; the chaincode function is given for
reference when calling `peer chaincode invoke` directly.

### Changes state

| Command | Chaincode function |
|---|---|
| `init-ledger` | `InitLedger` |
| `create-merchant-type <code> <name> [description]` | `CreateMerchantType` |
| `create-merchant <id> <name> <type> <taxId> [balance]` | `CreateMerchant` |
| `create-merchants --file <json>` | `CreateMerchants` |
| `change-merchant-type <merchantId> <newType>` | `ChangeMerchantType` |
| `create-customer <id> <first> <last> <email> [balance]` | `CreateCustomer` |
| `create-customers --file <json>` | `CreateCustomers` |
| `add-product <merchantId> <code> <name> <price> <qty> [expiry]` | `AddProduct` |
| `add-products <merchantId> --file <json>` | `AddProducts` |
| `update-price <code> <price>` | `UpdatePrice` |
| `restock <code> <qty>` | `RestockProduct` |
| `buy <customerId> <merchantId> <code> [qty]` | `BuyProduct` |
| `deposit <merchant\|customer> <id> <amount>` | `Deposit` |

### Read only

| Command | Chaincode function |
|---|---|
| `merchant-types`, `merchants`, `customers`, `products` | `GetAll...` |
| `merchant-type <code>`, `merchant <id>`, `customer <id>`, `product <code>`, `invoice <id>` | `Read...` |
| `search-name <text>` | `SearchProductsByName` |
| `search-code <text>` | `SearchProductsByCode` |
| `search-type <merchantType>` | `SearchProductsByMerchantType` |
| `search-price <from> <to>` | `SearchProductsByPrice` |
| `search [filters]` | `SearchProducts` |
| `search-paged [filters]` | `SearchProductsPaged` |
| `merchant-products <merchantId>` | `GetMerchantProducts` |
| `expiring <YYYY-MM-DD>` | `ProductsExpiringBefore` |
| `invoices --customer <id>` | `CustomerInvoices` |
| `invoices --customer <id> --min <n>` | `CustomerInvoicesAbove` |
| `invoices --merchant <id>` | `MerchantInvoices` |

---
