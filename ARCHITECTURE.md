# System Architecture

Trading system for arbitrary goods on a Hyperledger Fabric network. Course project
for PDASP 2024/25 (the assignment itself is in `specifikacija-projekta.md`).

This document records every decision made before a line of code was written:
naming, port allocation, data model and network rules. Every script and
configuration file that follows sticks to the names defined here.

---

## 1. Technology choices

| Layer | Choice | Rationale |
|---|---|---|
| Fabric | 2.5.x LTS | The assignment requires `>= 2.2.6`. 2.5 is the LTS line, ships the Peer Gateway service and the channel participation API (no system channel). |
| Channel creation | `osnadmin channel join` | The system channel is deprecated since 2.3. Each channel gets its own genesis block, handed directly to the orderers. |
| Consensus | RAFT (`etcdraft`), 3 nodes | Tolerates the loss of one node (quorum 2 of 3). The assignment asks for at least one orderer per channel; three is the correct RAFT setup. |
| State DB | CouchDB, one instance per peer | Mandated by the assignment. Enables rich queries over JSON document contents. |
| Identities | Fabric CA, 4 servers | Mandated by the assignment. `cryptogen` is deliberately not used. |
| Chaincode | Go + `fabric-contract-api-go` v2 | Recommended by the assignment. |
| Console application | Node.js + `@hyperledger/fabric-gateway` | SDK requirement. Talks gRPC to the Peer Gateway service, no connection profile needed. |

---

## 2. Network topology

```
                          +---------------------------------------+
                          |      Orderer org (OrdererMSP)         |
                          |   RAFT cluster, quorum 2 of 3         |
                          |  orderer.pdasp.com                    |
                          |  orderer2.pdasp.com                   |
                          |  orderer3.pdasp.com                   |
                          +-------------------+-------------------+
                                              |
              +-------------------------------+-------------------------------+
              |                               |                               |
     +--------+--------+            +---------+-------+            +----------+------+
     | Org1 (Org1MSP)  |            | Org2 (Org2MSP)  |            | Org3 (Org3MSP)  |
     |  "Merchants"    |            |  "Customers"    |            |  "Regulator"    |
     +-----------------+            +-----------------+            +-----------------+
     | peer0.org1      |            | peer0.org2      |            | peer0.org3      |
     | peer1.org1      |            | peer1.org2      |            | peer1.org3      |
     | peer2.org1      |            | peer2.org2      |            | peer2.org3      |
     |  + CouchDB x3   |            |  + CouchDB x3   |            |  + CouchDB x3   |
     |  + Fabric CA    |            |  + Fabric CA    |            |  + Fabric CA    |
     +-----------------+            +-----------------+            +-----------------+

     All nine peers join BOTH channels: `channel1` and `channel2`.
     Every peer is both endorser and committer (the Fabric default).
```

### 2.1 Domain and naming

The network domain is `pdasp.com` rather than the `example.com` used by
`fabric-samples`, so it is obvious the network was not copied from the samples.

| Element | Name |
|---|---|
| Organization MSPs | `Org1MSP`, `Org2MSP`, `Org3MSP`, `OrdererMSP` |
| Peers | `peer{0,1,2}.org{1,2,3}.pdasp.com` |
| Orderers | `orderer.pdasp.com`, `orderer2.pdasp.com`, `orderer3.pdasp.com` |
| CA servers | `ca.org{1,2,3}.pdasp.com`, `ca.orderer.pdasp.com` |
| CouchDB | `couchdb{0..8}` |
| Channels | `channel1`, `channel2` |
| Docker network | `pdasp_net` |
| Chaincode | `trade` |

### 2.2 Mapping organizations onto the business domain

The assignment explicitly leaves this mapping up to the student. The chosen one:

- **Org1, Merchants.** Identities from this org register merchants and products.
- **Org2, Customers.** Identities from this org register customers and make purchases.
- **Org3, Regulator.** Read-only in practice: used for searches and auditing.

The mapping is **not** enforced as an ACL inside the chaincode. That is deliberate:
the assignment states that many application-level merchants and customers may share
a single Fabric certificate. The split is organizational, and it is demonstrated by
the console application working with at least two certificates from different
organizations.

### 2.3 Channels

Both channels have identical configuration and run the same chaincode. The
difference is a business one:

- `channel1`, retail: supermarkets, pharmacies, electronics stores.
- `channel2`, specialized trade: auto parts, construction materials.

Ledgers are per channel, so data on `channel1` and `channel2` is completely
independent. The test suite uses this to show that the same transaction on two
channels produces two separate world states.

---

## 3. Network rules

### 3.1 Block cutting (assignment requirement)

```yaml
Orderer:
  BatchTimeout: 1s
  BatchSize:
    MaxMessageCount: 2
    AbsoluteMaxBytes: 10 MB
    PreferredMaxBytes: 2 MB
```

A block is cut once **2 transactions** have accumulated or after **1 second**,
whichever happens first.

### 3.2 Chaincode endorsement policy

```
OutOf(2, 'Org1MSP.peer', 'Org2MSP.peer', 'Org3MSP.peer')
```

Rationale for the defense: an `AND` over all three organizations would mean that
any one organization going down halts the whole system. An `OR` would let a single
organization write state on its own, which does not match a model where merchant
and customer are both parties to a purchase. `OutOf(2, ...)` is a majority: a
transaction succeeds when at least two of three organizations agree, and the system
survives the loss of one organization.

### 3.3 Channel and lifecycle policies

| Policy | Value |
|---|---|
| `Channel/Application/Readers` | `ANY Readers` |
| `Channel/Application/Writers` | `ANY Writers` |
| `Channel/Application/Admins` | `MAJORITY Admins` |
| `Channel/Application/LifecycleEndorsement` | `MAJORITY Endorsement` |
| `Channel/Application/Endorsement` | `MAJORITY Endorsement` |

`LifecycleEndorsement = MAJORITY` means at least two of three organizations must
approve a chaincode definition before it can be committed, which is why the deploy
script runs `approveformyorg` for all three.

---

## 4. Port map

Everything is published to the host so it can be inspected from the laptop
(Fauxton, operations endpoints, direct `peer` calls).

### 4.1 Fabric CA

| Service | CA port | Operations |
|---|---|---|
| `ca.org1.pdasp.com` | 7054 | 17054 |
| `ca.org2.pdasp.com` | 8054 | 18054 |
| `ca.org3.pdasp.com` | 9054 | 19054 |
| `ca.orderer.pdasp.com` | 10054 | 20054 |

### 4.2 Orderers

| Service | General | Admin (osnadmin) | Operations |
|---|---|---|---|
| `orderer.pdasp.com` | 7050 | 7053 | 9440 |
| `orderer2.pdasp.com` | 7060 | 7063 | 9441 |
| `orderer3.pdasp.com` | 7070 | 7073 | 9442 |

RAFT traffic between orderers travels over the same port as client traffic. A
separate cluster listener is unnecessary when TLS is enabled, so none is configured.

### 4.3 Peers

| Peer | Peer port | Chaincode | Operations | CouchDB (host) |
|---|---|---|---|---|
| `peer0.org1.pdasp.com` | 7051 | 7052* | 9451 | 5984 |
| `peer1.org1.pdasp.com` | 7151 | 7152* | 9452 | 5985 |
| `peer2.org1.pdasp.com` | 7251 | 7252* | 9453 | 5986 |
| `peer0.org2.pdasp.com` | 8051 | 8052* | 9454 | 5987 |
| `peer1.org2.pdasp.com` | 8151 | 8152* | 9455 | 5988 |
| `peer2.org2.pdasp.com` | 8251 | 8252* | 9456 | 5989 |
| `peer0.org3.pdasp.com` | 9051 | 9052* | 9457 | 5990 |
| `peer1.org3.pdasp.com` | 9151 | 9152* | 9458 | 5991 |
| `peer2.org3.pdasp.com` | 9251 | 9252* | 9459 | 5992 |

\* the chaincode port is internal (container to container) and is not published.

The **anchor peer** of every organization is `peer0`. Anchor peers are baked into
the channel genesis block, so no separate anchor peer update transaction is needed.

---

## 5. Data model (world state)

Every object is stored as JSON. Every document carries a `docType` field, which is
what makes CouchDB rich queries possible: all types share one database, so without
it there would be no way to select only products or only invoices.

### 5.1 Keys

Keys are prefixed by type so entities with the same identifier cannot collide:

```
TYPE_<code>           e.g. TYPE_SUPERMARKET
MERCHANT_<merchantId> e.g. MERCHANT_M001
PRODUCT_<code>        e.g. PRODUCT_P001
CUSTOMER_<customerId> e.g. CUSTOMER_C001
INVOICE_<id>          e.g. INVOICE_<txId>
```

### 5.2 Structures

```go
type MerchantType struct {
    DocType     string `json:"docType"` // "merchantType"
    Code        string `json:"code"`    // SUPERMARKET, AUTO_PARTS, PHARMACY, ELECTRONICS, CONSTRUCTION
    Name        string `json:"name"`
    Description string `json:"description"`
}

type Merchant struct {
    DocType    string   `json:"docType"`    // "merchant"
    MerchantId string   `json:"merchantId"`
    Name       string   `json:"name"`
    Type       string   `json:"type"`       // references MerchantType.Code
    TaxId      string   `json:"taxId"`      // PIB
    Products   []string `json:"products"`   // product codes on offer
    Invoices   []string `json:"invoices"`   // issued invoice ids
    Balance    float64  `json:"balance"`
}

type Product struct {
    DocType      string  `json:"docType"`      // "product"
    Code         string  `json:"code"`
    Name         string  `json:"name"`
    ExpiryDate   string  `json:"expiryDate"`   // YYYY-MM-DD, empty when not applicable
    Price        float64 `json:"price"`
    Quantity     int     `json:"quantity"`
    MerchantId   string  `json:"merchantId"`
    MerchantType string  `json:"merchantType"` // denormalized, see 5.3
}

type Customer struct {
    DocType    string   `json:"docType"` // "customer"
    CustomerId string   `json:"customerId"`
    FirstName  string   `json:"firstName"`
    LastName   string   `json:"lastName"`
    Email      string   `json:"email"`
    Invoices   []string `json:"invoices"`
    Balance    float64  `json:"balance"`
}

type Invoice struct {
    DocType     string  `json:"docType"` // "invoice"
    Id          string  `json:"id"`
    MerchantId  string  `json:"merchantId"`
    CustomerId  string  `json:"customerId"`
    ProductCode string  `json:"productCode"`
    ProductName string  `json:"productName"` // snapshot, the product may be deleted
    Quantity    int     `json:"quantity"`
    UnitPrice   float64 `json:"unitPrice"`
    Total       float64 `json:"total"`
    Date        string  `json:"date"` // RFC3339, taken from TxTimestamp
}
```

### 5.3 Denormalizing `merchantType` onto the product

The assignment requires searching products **by merchant type**. Merchant type
physically belongs to the merchant document, not to the product. Without
denormalization that search needs two passes: first find merchants of the given
type, then filter products by their ids. In CouchDB that means two queries plus a
join in application code.

So `merchantType` is written onto the product as well. A **combined** search (name
plus merchant type plus price range) then becomes a **single** CouchDB selector.
The price of consistency is that changing a merchant's type must propagate to all
of its products, which is what `ChangeMerchantType` does.

### 5.4 Determinism

The chaincode must never call `time.Now()` or use randomness: different endorsers
would produce different read-write sets and endorsement would fail. The invoice date
comes from `ctx.GetStub().GetTxTimestamp()` and the invoice id from
`ctx.GetStub().GetTxID()`, both identical across all endorsers.

---

## 6. CouchDB rich queries (required for the defense)

Indexes live in `chaincode/trade/META-INF/statedb/couchdb/indexes/`.

Representative queries:

1. **Combined search**, name contains a term, merchant type from a set, price in a range:
   ```json
   {"selector":{"docType":"product",
                "name":{"$regex":"(?i)milk"},
                "merchantType":{"$in":["SUPERMARKET","PHARMACY"]},
                "price":{"$gte":50,"$lte":300}}}
   ```
2. **Products expiring before a date that are still in stock:**
   ```json
   {"selector":{"docType":"product",
                "expiryDate":{"$gt":"","$lte":"2026-10-01"},
                "quantity":{"$gt":0}}}
   ```
3. **One customer's invoices above an amount:**
   ```json
   {"selector":{"docType":"invoice","customerId":"C001","total":{"$gte":1000}}}
   ```

**Why this is impossible on LevelDB.** LevelDB is a pure key-value store. It
supports only `GetState(key)` and `GetStateByRange(start, end)`, that is, lookup by
**key** or by **lexicographic key range**. To LevelDB the value is an opaque byte
array; it cannot filter on `price` because it does not know the value has a `price`
field at all.

Reproducing the same functionality on LevelDB would require hand-built secondary
indexes using composite keys (`CreateCompositeKey("type~price~code", ...)`), and:

- every field combination that can be searched needs its **own** composite key;
- every `PutState` must maintain all of those indexes and delete the stale entries;
- a price range only works if the price is stored in the key as a zero-padded
  fixed-width string;
- `$regex` over a name and `$in` over a set of types **cannot be expressed** at all.
  The whole range would have to be scanned and filtered in Go, which is O(n) over
  the entire ledger.

Query 1 above would need either N separate composite key indexes, one per subset of
filters, or a full scan. CouchDB solves it with a single selector.

One caveat worth stating on the defense: the result of a rich query is **not**
re-validated at commit time, because the read set contains only keys and not the
query predicate. That is why these queries are only ever used as queries
(`evaluateTransaction`), never as the basis for a write.

---

## 7. Repository layout

```
PDASP/
|- ARCHITECTURE.md            <- this document
|- README.md                  <- how to run everything
|- specifikacija-projekta.md  <- the original assignment (Serbian)
|- vodic-fabric-projekat.md   <- initial notes
|
|- network/                   <- everything about the Fabric network
|  |- install-fabric.sh       <- downloads binaries and docker images
|  |- network-up.sh
|  |- network-down.sh
|  |- create-channels.sh
|  |- deploy-chaincode.sh
|  |- .env                    <- versions, names, ports
|  |- configtx/configtx.yaml
|  |- compose/
|  |  |- compose-ca.yaml
|  |  \- compose-net.yaml
|  |- scripts/
|  |  |- utils.sh             <- logging, checks, waiting
|  |  |- env-var.sh           <- CORE_PEER_* setup for the peer CLI
|  |  \- register-enroll.sh
|  |- organizations/          <- GENERATED (gitignored)
|  |- channel-artifacts/      <- GENERATED (gitignored)
|  |- bin/                    <- Fabric binaries (gitignored)
|  \- config/                 <- Fabric default configs (gitignored)
|
|- chaincode/trade/           <- Go chaincode
|  |- go.mod
|  |- main.go
|  |- META-INF/statedb/couchdb/indexes/*.json
|  \- contract/
|     |- model.go
|     |- keys.go
|     |- errors.go
|     |- state.go
|     |- validation.go
|     |- init.go
|     |- merchant_type.go
|     |- merchant.go
|     |- customer.go
|     |- product.go
|     |- purchase.go
|     |- queries.go
|     \- *_test.go
|
|- application/               <- Node.js console application
|  |- package.json
|  |- src/
|  \- wallet/                 <- GENERATED (gitignored)
|
\- test/                      <- shell test scripts
   |- run-all-tests.sh
   \- test-*.sh
```

---

## 8. Startup sequence

```
./network/install-fabric.sh      # one time: binaries and images
./network/network-up.sh          # CA -> crypto material -> genesis -> orderer/peer/couchdb
./network/create-channels.sh     # channel1 and channel2, join all peers, anchor peers
./network/deploy-chaincode.sh    # package/install/approve/commit + InitLedger
cd application && npm install
node src/index.js                # interactive menu
./test/run-all-tests.sh          # full regression pass
```

---

## 9. Commit plan

| # | Commit | Area |
|---|---|---|
| 01 | repository skeleton, `.gitignore`, architecture | infra |
| 02 | `install-fabric.sh` | network |
| 03 | Fabric CA compose | network |
| 04 | `register-enroll.sh` | network |
| 05 | `configtx.yaml` (RAFT, block cutting, channel profile) | network |
| 06 | `compose-net.yaml` (orderer/peer/couchdb) | network |
| 07 | `network-up.sh` / `network-down.sh` | network |
| 08 | `create-channels.sh` | network |
| 09 | chaincode: module, model, keys | chaincode |
| 10 | chaincode: `InitLedger` and entity creation | chaincode |
| 11 | chaincode: purchase and deposit | chaincode |
| 12 | chaincode: rich queries and CouchDB indexes | chaincode |
| 13 | chaincode: unit tests | chaincode |
| 14 | `deploy-chaincode.sh` | network |
| 15 | application: skeleton, CA enrollment, gateway connection | application |
| 16 | application: invoke actions | application |
| 17 | application: query actions and menu | application |
| 18 | shell test scripts | tests |
| 19 | README, diagram, defense preparation | docs |
