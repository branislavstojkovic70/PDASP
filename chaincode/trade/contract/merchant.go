package contract

import (
	"encoding/json"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)


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

type MerchantInput struct {
	MerchantId     string  `json:"merchantId"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	TaxId          string  `json:"taxId"`
	OpeningBalance float64 `json:"openingBalance"`
}


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

	seen := map[string]bool{}
	created := make([]*Merchant, 0, len(inputs))
	for _, input := range inputs {
		if seen[input.MerchantId] {
			return nil, errDuplicateInBatch("merchant", input.MerchantId)
		}
		seen[input.MerchantId] = true

		merchant, err := c.CreateMerchant(ctx, input.MerchantId, input.Name, input.Type,
			input.TaxId, input.OpeningBalance)
		if err != nil {
			return nil, err
		}
		created = append(created, merchant)
	}
	return created, nil
}

func (c *TradeContract) ReadMerchant(
	ctx contractapi.TransactionContextInterface, merchantId string,
) (*Merchant, error) {
	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	return loadMerchant(ctx, merchantId)
}

func (c *TradeContract) GetAllMerchants(
	ctx contractapi.TransactionContextInterface,
) ([]*Merchant, error) {
	return readByPrefix[Merchant](ctx, prefixMerchant)
}

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
			continue 
		}
		product.MerchantType = newType
		if err := writeState(ctx, productKey(code), product); err != nil {
			return nil, err
		}
	}
	return merchant, nil
}
