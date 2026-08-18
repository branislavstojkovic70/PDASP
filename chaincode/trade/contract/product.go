package contract

import (
	"encoding/json"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

func (c *TradeContract) AddProduct(
	ctx contractapi.TransactionContextInterface,
	merchantId string, code string, name string, expiryDate string,
	price float64, quantity int,
) (*Product, error) {

	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	merchant, err := loadMerchant(ctx, merchantId)
	if err != nil {
		return nil, err
	}

	product, err := buildProduct(ctx, merchant, ProductInput{
		Code:       code,
		Name:       name,
		ExpiryDate: expiryDate,
		Price:      price,
		Quantity:   quantity,
	})
	if err != nil {
		return nil, err
	}

	merchant.Products = appendUnique(merchant.Products, product.Code)
	if err := writeState(ctx, merchantKey(merchant.MerchantId), merchant); err != nil {
		return nil, err
	}
	return product, nil
}

type ProductInput struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	ExpiryDate string  `json:"expiryDate"`
	Price      float64 `json:"price"`
	Quantity   int     `json:"quantity"`
}


func (c *TradeContract) AddProducts(
	ctx contractapi.TransactionContextInterface,
	merchantId string, productsJSON string,
) ([]*Product, error) {

	merchantId, err := requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	productsJSON, err = requireText("productsJSON", productsJSON)
	if err != nil {
		return nil, err
	}

	var inputs []ProductInput
	if err := json.Unmarshal([]byte(productsJSON), &inputs); err != nil {
		return nil, errInvalidJSON("the product list", err)
	}
	if len(inputs) == 0 {
		return nil, errEmptyParameter("productsJSON (empty array)")
	}

	merchant, err := loadMerchant(ctx, merchantId)
	if err != nil {
		return nil, err
	}


	seen := map[string]bool{}
	added := make([]*Product, 0, len(inputs))
	for _, input := range inputs {
		if seen[input.Code] {
			return nil, errDuplicateInBatch("product", input.Code)
		}
		seen[input.Code] = true

		product, err := buildProduct(ctx, merchant, input)
		if err != nil {
			return nil, err
		}
		merchant.Products = appendUnique(merchant.Products, product.Code)
		added = append(added, product)
	}

	if err := writeState(ctx, merchantKey(merchant.MerchantId), merchant); err != nil {
		return nil, err
	}
	return added, nil
}

func (c *TradeContract) ReadProduct(
	ctx contractapi.TransactionContextInterface, code string,
) (*Product, error) {
	code, err := requireText("code", code)
	if err != nil {
		return nil, err
	}
	return loadProduct(ctx, code)
}

func (c *TradeContract) GetAllProducts(
	ctx contractapi.TransactionContextInterface,
) ([]*Product, error) {
	return readByPrefix[Product](ctx, prefixProduct)
}

func (c *TradeContract) UpdatePrice(
	ctx contractapi.TransactionContextInterface, code string, newPrice float64,
) (*Product, error) {

	code, err := requireText("code", code)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveAmount("newPrice", newPrice); err != nil {
		return nil, err
	}

	product, err := loadProduct(ctx, code)
	if err != nil {
		return nil, err
	}
	product.Price = roundMoney(newPrice)
	if err := writeState(ctx, productKey(code), product); err != nil {
		return nil, err
	}
	return product, nil
}

func (c *TradeContract) RestockProduct(
	ctx contractapi.TransactionContextInterface, code string, additionalQuantity int,
) (*Product, error) {

	code, err := requireText("code", code)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveQuantity("additionalQuantity", additionalQuantity); err != nil {
		return nil, err
	}

	product, err := loadProduct(ctx, code)
	if err != nil {
		return nil, err
	}
	product.Quantity += additionalQuantity
	if err := writeState(ctx, productKey(code), product); err != nil {
		return nil, err
	}
	return product, nil
}

func buildProduct(
	ctx contractapi.TransactionContextInterface, merchant *Merchant, input ProductInput,
) (*Product, error) {

	code, err := requireText("code", input.Code)
	if err != nil {
		return nil, err
	}
	name, err := requireText("name", input.Name)
	if err != nil {
		return nil, err
	}
	expiry, err := parseExpiryDate(input.ExpiryDate)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveAmount("price", input.Price); err != nil {
		return nil, err
	}
	if err := requirePositiveQuantity("quantity", input.Quantity); err != nil {
		return nil, err
	}

	key := productKey(code)
	exists, err := stateExists(ctx, key)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errProductExists(code)
	}

	product := &Product{
		DocType:      DocProduct,
		Code:         code,
		Name:         name,
		ExpiryDate:   expiry,
		Price:        roundMoney(input.Price),
		Quantity:     input.Quantity,
		MerchantId:   merchant.MerchantId,
		MerchantType: merchant.Type,
	}
	if err := writeState(ctx, key, product); err != nil {
		return nil, err
	}
	return product, nil
}
