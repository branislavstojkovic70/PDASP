package contract

import (
	"encoding/json"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)


type TradeContract struct {
	contractapi.Contract
}

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

func deleteState(ctx contractapi.TransactionContextInterface, key string) error {
	if err := ctx.GetStub().DelState(key); err != nil {
		return errDelete(key, err)
	}
	return nil
}

func stateExists(ctx contractapi.TransactionContextInterface, key string) (bool, error) {
	bytes, err := ctx.GetStub().GetState(key)
	if err != nil {
		return false, errRead(key, err)
	}
	return bytes != nil, nil
}

func readByPrefix[T any](ctx contractapi.TransactionContextInterface, prefix string) ([]*T, error) {
	from, to := prefixRange(prefix)
	iterator, err := ctx.GetStub().GetStateByRange(from, to)
	if err != nil {
		return nil, errQuery(err)
	}
	defer iterator.Close()
	return readIterator[T](iterator)
}

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


func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}


func removeValue(list []string, value string) ([]string, bool) {
	for i, existing := range list {
		if existing == value {
			return append(list[:i:i], list[i+1:]...), true
		}
	}
	return list, false
}

func contains(list []string, value string) bool {
	for _, existing := range list {
		if existing == value {
			return true
		}
	}
	return false
}
