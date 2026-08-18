package contract

import "github.com/hyperledger/fabric-contract-api-go/v2/contractapi"


func (c *TradeContract) CreateMerchantType(
	ctx contractapi.TransactionContextInterface,
	code string, name string, description string,
) (*MerchantType, error) {

	code, err := requireText("code", code)
	if err != nil {
		return nil, err
	}
	name, err = requireText("name", name)
	if err != nil {
		return nil, err
	}

	key := merchantTypeKey(code)
	exists, err := stateExists(ctx, key)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errMerchantTypeExists(code)
	}

	merchantType := &MerchantType{
		DocType:     DocMerchantType,
		Code:        code,
		Name:        name,
		Description: description,
	}
	if err := writeState(ctx, key, merchantType); err != nil {
		return nil, err
	}
	return merchantType, nil
}

// ReadMerchantType returns a single catalogue entry.
func (c *TradeContract) ReadMerchantType(
	ctx contractapi.TransactionContextInterface, code string,
) (*MerchantType, error) {
	code, err := requireText("code", code)
	if err != nil {
		return nil, err
	}
	return loadMerchantType(ctx, code)
}

// GetAllMerchantTypes returns the whole catalogue.
//
// This deliberately walks a key range instead of issuing a CouchDB query: the
// catalogue is small and looked up by key, so a rich query would add nothing. The
// same result would be produced on LevelDB.
func (c *TradeContract) GetAllMerchantTypes(
	ctx contractapi.TransactionContextInterface,
) ([]*MerchantType, error) {
	return readByPrefix[MerchantType](ctx, prefixMerchantType)
}
