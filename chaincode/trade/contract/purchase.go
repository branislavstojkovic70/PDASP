package contract

import (
	"strings"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// PurchaseResult is the outcome of a successful purchase.
//
// It carries the new balances and the remaining quantity so the console
// application does not need extra queries just to show the effect of the
// transaction.
type PurchaseResult struct {
	Invoice           *Invoice `json:"invoice"`
	CustomerBalance   float64  `json:"customerBalance"`
	MerchantBalance   float64  `json:"merchantBalance"`
	RemainingQuantity int      `json:"remainingQuantity"`
	ProductRemoved    bool     `json:"productRemoved"`
}

// BuyProduct performs a purchase of a product from a merchant.
//
// The order is deliberate: every check runs before the first write. A transaction
// cannot be rolled back, so a half applied purchase (money taken but no goods
// reserved) would stay in the ledger forever.
//
// When the quantity reaches zero the product is deleted from the world state and
// from the merchant's offering, as the assignment requires. The invoice remains
// readable because it carries a snapshot of the name, unit price and quantity.
func (c *TradeContract) BuyProduct(
	ctx contractapi.TransactionContextInterface,
	customerId string, merchantId string, productCode string, quantity int,
) (*PurchaseResult, error) {

	customerId, err := requireText("customerId", customerId)
	if err != nil {
		return nil, err
	}
	merchantId, err = requireText("merchantId", merchantId)
	if err != nil {
		return nil, err
	}
	productCode, err = requireText("productCode", productCode)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveQuantity("quantity", quantity); err != nil {
		return nil, err
	}

	// --- load the participants ----------------------------------------------
	customer, err := loadCustomer(ctx, customerId)
	if err != nil {
		return nil, err
	}
	merchant, err := loadMerchant(ctx, merchantId)
	if err != nil {
		return nil, err
	}
	product, err := loadProduct(ctx, productCode)
	if err != nil {
		return nil, err
	}

	// --- business checks -----------------------------------------------------
	// The product exists but may belong to a different merchant. Both the field on
	// the product and the merchant's offering are checked, because those are two
	// records that could in principle drift apart.
	if product.MerchantId != merchantId || !contains(merchant.Products, productCode) {
		return nil, errProductNotSoldByMerchant(productCode, merchantId)
	}
	if product.Quantity < quantity {
		return nil, errInsufficientStock(productCode, product.Quantity, quantity)
	}

	total := roundMoney(product.Price * float64(quantity))
	if customer.Balance < total {
		return nil, errInsufficientFunds(customerId, customer.Balance, total)
	}

	// --- create the invoice --------------------------------------------------
	// The id and the date must be identical on every endorser, otherwise the
	// read-write sets diverge and endorsement fails. Hence TxID and TxTimestamp,
	// never uuid.New() or time.Now().
	date, err := transactionTime(ctx)
	if err != nil {
		return nil, err
	}
	invoice := &Invoice{
		DocType:     DocInvoice,
		Id:          ctx.GetStub().GetTxID(),
		MerchantId:  merchantId,
		CustomerId:  customerId,
		ProductCode: productCode,
		ProductName: product.Name,
		Quantity:    quantity,
		UnitPrice:   product.Price,
		Total:       total,
		Date:        date,
	}

	// --- apply the changes ---------------------------------------------------
	customer.Balance = roundMoney(customer.Balance - total)
	merchant.Balance = roundMoney(merchant.Balance + total)
	customer.Invoices = append(customer.Invoices, invoice.Id)
	merchant.Invoices = append(merchant.Invoices, invoice.Id)
	product.Quantity -= quantity

	removed := product.Quantity == 0
	if removed {
		if err := deleteState(ctx, productKey(productCode)); err != nil {
			return nil, err
		}
		merchant.Products, _ = removeValue(merchant.Products, productCode)
	} else if err := writeState(ctx, productKey(productCode), product); err != nil {
		return nil, err
	}

	if err := writeState(ctx, invoiceKey(invoice.Id), invoice); err != nil {
		return nil, err
	}
	if err := writeState(ctx, customerKey(customerId), customer); err != nil {
		return nil, err
	}
	if err := writeState(ctx, merchantKey(merchantId), merchant); err != nil {
		return nil, err
	}

	return &PurchaseResult{
		Invoice:           invoice,
		CustomerBalance:   customer.Balance,
		MerchantBalance:   merchant.Balance,
		RemainingQuantity: product.Quantity,
		ProductRemoved:    removed,
	}, nil
}

// DepositResult is the outcome of a deposit.
type DepositResult struct {
	EntityType string  `json:"entityType"`
	Id         string  `json:"id"`
	Deposited  float64 `json:"deposited"`
	OldBalance float64 `json:"oldBalance"`
	NewBalance float64 `json:"newBalance"`
}

// Deposit increases the balance of a merchant or a customer.
//
// entityType is "merchant" or "customer". One function rather than two, because
// the logic is identical and the console application asks who is being credited
// anyway.
func (c *TradeContract) Deposit(
	ctx contractapi.TransactionContextInterface,
	entityType string, id string, amount float64,
) (*DepositResult, error) {

	entityType, err := requireText("entityType", entityType)
	if err != nil {
		return nil, err
	}
	id, err = requireText("id", id)
	if err != nil {
		return nil, err
	}
	if err := requirePositiveAmount("amount", amount); err != nil {
		return nil, err
	}

	amount = roundMoney(amount)
	result := &DepositResult{EntityType: strings.ToLower(entityType), Id: id, Deposited: amount}

	switch result.EntityType {
	case "merchant":
		merchant, err := loadMerchant(ctx, id)
		if err != nil {
			return nil, err
		}
		result.OldBalance = merchant.Balance
		merchant.Balance = roundMoney(merchant.Balance + amount)
		result.NewBalance = merchant.Balance
		if err := writeState(ctx, merchantKey(id), merchant); err != nil {
			return nil, err
		}

	case "customer":
		customer, err := loadCustomer(ctx, id)
		if err != nil {
			return nil, err
		}
		result.OldBalance = customer.Balance
		customer.Balance = roundMoney(customer.Balance + amount)
		result.NewBalance = customer.Balance
		if err := writeState(ctx, customerKey(id), customer); err != nil {
			return nil, err
		}

	default:
		return nil, errUnknownEntityType(entityType)
	}

	return result, nil
}

// ReadInvoice returns a single invoice by identifier.
func (c *TradeContract) ReadInvoice(
	ctx contractapi.TransactionContextInterface, id string,
) (*Invoice, error) {
	id, err := requireText("id", id)
	if err != nil {
		return nil, err
	}
	invoice, found, err := readState[Invoice](ctx, invoiceKey(id))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errInvoiceNotFound(id)
	}
	return invoice, nil
}

// transactionTime returns the transaction time in RFC3339 form.
//
// It comes from the transaction proposal rather than the node clock: every
// endorser must compute exactly the same result.
func transactionTime(ctx contractapi.TransactionContextInterface) (string, error) {
	stamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", errTxTimestamp(err)
	}
	return stamp.AsTime().UTC().Format(time.RFC3339), nil
}
