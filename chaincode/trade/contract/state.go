package contract

import (
	"encoding/json"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// TradeContract is the smart contract of the trading system.
//
// It performs no per organization access control: the assignment allows many
// application level merchants and customers to share one Fabric certificate, so
// the split into Org1 (merchants), Org2 (customers) and Org3 (regulator) is
// organizational rather than technical. Who may write state is governed by the
// channel endorsement policy.
type TradeContract struct {
	contractapi.Contract
}

// ---------------------------------------------------------------------------
// Generic world state access.
//
// Every structure is serialized as JSON (an assignment requirement), so reading
// and writing follow the same shape for every entity type.
// ---------------------------------------------------------------------------

// readState loads a document from a key. The second result is false when the key
// does not exist, which is not an error but a state callers frequently expect,
// for example when checking for duplicates before an insert.
func readState[T any](ctx contractapi.TransactionContextInterface, key string) (*T, bool, error) {
	bytes, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, false, errRead(key, err)
	}
	if bytes == nil {
		return nil, false, nil
	}

	var value T
	if err := json.Unmarshal(bytes, &value); err != nil {
		return nil, false, errInvalidJSON(key, err)
	}
	return &value, true, nil
}

// writeState serializes a document to JSON and stores it under a key.
func writeState(ctx contractapi.TransactionContextInterface, key string, value any) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return errInvalidJSON(key, err)
	}
	if err := ctx.GetStub().PutState(key, bytes); err != nil {
		return errWrite(key, err)
	}
	return nil
}

// deleteState removes a key from the world state.
func deleteState(ctx contractapi.TransactionContextInterface, key string) error {
	if err := ctx.GetStub().DelState(key); err != nil {
		return errDelete(key, err)
	}
	return nil
}

// stateExists checks for the presence of a key without deserializing it.
func stateExists(ctx contractapi.TransactionContextInterface, key string) (bool, error) {
	bytes, err := ctx.GetStub().GetState(key)
	if err != nil {
		return false, errRead(key, err)
	}
	return bytes != nil, nil
}

// readByPrefix returns every document whose key starts with the given prefix.
//
// This is the only kind of lookup that would also work on LevelDB: walking a range
// of keys. Filtering on document content is not possible here; see queries.go for
// the CouchDB rich query variant.
func readByPrefix[T any](ctx contractapi.TransactionContextInterface, prefix string) ([]*T, error) {
	from, to := prefixRange(prefix)
	iterator, err := ctx.GetStub().GetStateByRange(from, to)
	if err != nil {
		return nil, errQuery(err)
	}
	defer iterator.Close()
	return readIterator[T](iterator)
}

// readIterator deserializes every record from an iterator into a slice.
// GetStateByRange, GetQueryResult and GetQueryResultWithPagination all return the
// same iterator type, so one function covers all three cases.
func readIterator[T any](iterator shim.StateQueryIteratorInterface) ([]*T, error) {
	result := []*T{}
	for iterator.HasNext() {
		record, err := iterator.Next()
		if err != nil {
			return nil, errQuery(err)
		}
		var value T
		if err := json.Unmarshal(record.Value, &value); err != nil {
			return nil, errInvalidJSON(record.Key, err)
		}
		result = append(result, &value)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Loading concrete entities, erroring when they do not exist.
// ---------------------------------------------------------------------------

func loadMerchant(ctx contractapi.TransactionContextInterface, id string) (*Merchant, error) {
	merchant, found, err := readState[Merchant](ctx, merchantKey(id))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errMerchantNotFound(id)
	}
	return merchant, nil
}

func loadCustomer(ctx contractapi.TransactionContextInterface, id string) (*Customer, error) {
	customer, found, err := readState[Customer](ctx, customerKey(id))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errCustomerNotFound(id)
	}
	return customer, nil
}

func loadProduct(ctx contractapi.TransactionContextInterface, code string) (*Product, error) {
	product, found, err := readState[Product](ctx, productKey(code))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errProductNotFound(code)
	}
	return product, nil
}

func loadMerchantType(ctx contractapi.TransactionContextInterface, code string) (*MerchantType, error) {
	merchantType, found, err := readState[MerchantType](ctx, merchantTypeKey(code))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errMerchantTypeNotFound(code)
	}
	return merchantType, nil
}

// ---------------------------------------------------------------------------
// Working with identifier lists inside documents.
// ---------------------------------------------------------------------------

// appendUnique adds a value to a list unless it is already present.
func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

// removeValue drops a value from a list. It returns the new list and whether
// anything was removed. The original slice is never modified.
func removeValue(list []string, value string) ([]string, bool) {
	for i, existing := range list {
		if existing == value {
			return append(list[:i:i], list[i+1:]...), true
		}
	}
	return list, false
}

// contains reports whether a value is present in a list.
func contains(list []string, value string) bool {
	for _, existing := range list {
		if existing == value {
			return true
		}
	}
	return false
}
