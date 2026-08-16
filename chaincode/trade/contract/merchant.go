package contract

import (
	"encoding/json"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// CreateMerchant stores a new merchant.
//
// The type must exist in the catalogue (see merchant_type.go). Without that check
// merchants with arbitrary types would leak into the world state and searching
// products by merchant type would return incomplete results.
func (c *TradeContract) CreateMerchant(
	ctx contractapi.TransactionContextInterface,
	merchantId string, name string, merchantType string, taxId string, openingBalance float64,
) (*Merchant, error) {

	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	name, err = requireText("name", name)
	if err != nil {
		return nil, err
	}
	merchantType, err = requireText("merchantType", merchantType)
	if err != nil {
		return nil, err
	}
	taxId, err = requireText("taxId", taxId)
	if err != nil {
		return nil, err
	}
	if openingBalance < 0 {
		return nil, errNonPositiveAmount("openingBalance", openingBalance)
	}

	key := merchantKey(merchantId)
	exists, err := stateExists(ctx, key)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errMerchantExists(merchantId)
	}

	if _, err := loadMerchantType(ctx, merchantType); err != nil {
		return nil, err
	}

	merchant := &Merchant{
		DocType:    DocMerchant,
		MerchantId: merchantId,
		Name:       name,
		Type:       merchantType,
		TaxId:      taxId,
		Products:   []string{},
		Invoices:   []string{},
		Balance:    roundMoney(openingBalance),
	}
	if err := writeState(ctx, key, merchant); err != nil {
		return nil, err
	}
	return merchant, nil
}

// MerchantInput is one element of the JSON array accepted by CreateMerchants.
type MerchantInput struct {
	MerchantId     string  `json:"merchantId"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	TaxId          string  `json:"taxId"`
	OpeningBalance float64 `json:"openingBalance"`
}

// CreateMerchants stores several merchants at once from a JSON array.
//
// The whole call is one transaction: if any merchant fails validation none of them
// is stored. That avoids partial state, which cannot be undone.
func (c *TradeContract) CreateMerchants(
	ctx contractapi.TransactionContextInterface, merchantsJSON string,
) ([]*Merchant, error) {

	merchantsJSON, err := requireText("merchantsJSON", merchantsJSON)
	if err != nil {
		return nil, err
	}

	var inputs []MerchantInput
	if err := json.Unmarshal([]byte(merchantsJSON), &inputs); err != nil {
		return nil, errInvalidJSON("the merchant list", err)
	}
	if len(inputs) == 0 {
		return nil, errEmptyParameter("merchantsJSON (empty array)")
	}

	created := make([]*Merchant, 0, len(inputs))
	for _, input := range inputs {
		merchant, err := c.CreateMerchant(ctx, input.MerchantId, input.Name, input.Type,
			input.TaxId, input.OpeningBalance)
		if err != nil {
			return nil, err
		}
		created = append(created, merchant)
	}
	return created, nil
}

// ReadMerchant returns a single merchant by identifier.
func (c *TradeContract) ReadMerchant(
	ctx contractapi.TransactionContextInterface, merchantId string,
) (*Merchant, error) {
	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	return loadMerchant(ctx, merchantId)
}

// GetAllMerchants returns every merchant, walking the key range.
func (c *TradeContract) GetAllMerchants(
	ctx contractapi.TransactionContextInterface,
) ([]*Merchant, error) {
	return readByPrefix[Merchant](ctx, prefixMerchant)
}

// ChangeMerchantType changes a merchant's line of business and propagates the
// change to all of its products.
//
// The propagation is the price of denormalizing merchantType onto Product (see
// model.go): without it the products would keep the old type and searching by
// merchant type would return wrong results.
func (c *TradeContract) ChangeMerchantType(
	ctx contractapi.TransactionContextInterface, merchantId string, newType string,
) (*Merchant, error) {

	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	newType, err = requireText("newType", newType)
	if err != nil {
		return nil, err
	}

	merchant, err := loadMerchant(ctx, merchantId)
	if err != nil {
		return nil, err
	}
	if _, err := loadMerchantType(ctx, newType); err != nil {
		return nil, err
	}

	merchant.Type = newType
	if err := writeState(ctx, merchantKey(merchantId), merchant); err != nil {
		return nil, err
	}

	for _, code := range merchant.Products {
		product, found, err := readState[Product](ctx, productKey(code))
		if err != nil {
			return nil, err
		}
		if !found {
			continue // the product sold out and was deleted in the meantime
		}
		product.MerchantType = newType
		if err := writeState(ctx, productKey(code), product); err != nil {
			return nil, err
		}
	}
	return merchant, nil
}
