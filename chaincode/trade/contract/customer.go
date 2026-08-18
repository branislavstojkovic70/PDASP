package contract

import (
	"encoding/json"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

func (c *TradeContract) CreateCustomer(
	ctx contractapi.TransactionContextInterface,
	customerId string, firstName string, lastName string, email string, openingBalance float64,
) (*Customer, error) {

	customerId, err := requireText("customerId", customerId)
	if err != nil {
		return nil, err
	}
	firstName, err = requireText("firstName", firstName)
	if err != nil {
		return nil, err
	}
	lastName, err = requireText("lastName", lastName)
	if err != nil {
		return nil, err
	}
	email, err = requireEmail(email)
	if err != nil {
		return nil, err
	}
	if openingBalance < 0 {
		return nil, errNonPositiveAmount("openingBalance", openingBalance)
	}

	key := customerKey(customerId)
	exists, err := stateExists(ctx, key)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errCustomerExists(customerId)
	}

	customer := &Customer{
		DocType:    DocCustomer,
		CustomerId: customerId,
		FirstName:  firstName,
		LastName:   lastName,
		Email:      email,
		Invoices:   []string{},
		Balance:    roundMoney(openingBalance),
	}
	if err := writeState(ctx, key, customer); err != nil {
		return nil, err
	}
	return customer, nil
}

type CustomerInput struct {
	CustomerId     string  `json:"customerId"`
	FirstName      string  `json:"firstName"`
	LastName       string  `json:"lastName"`
	Email          string  `json:"email"`
	OpeningBalance float64 `json:"openingBalance"`
}


func (c *TradeContract) CreateCustomers(
	ctx contractapi.TransactionContextInterface, customersJSON string,
) ([]*Customer, error) {

	customersJSON, err := requireText("customersJSON", customersJSON)
	if err != nil {
		return nil, err
	}

	var inputs []CustomerInput
	if err := json.Unmarshal([]byte(customersJSON), &inputs); err != nil {
		return nil, errInvalidJSON("the customer list", err)
	}
	if len(inputs) == 0 {
		return nil, errEmptyParameter("customersJSON (empty array)")
	}

	seen := map[string]bool{}
	created := make([]*Customer, 0, len(inputs))
	for _, input := range inputs {
		if seen[input.CustomerId] {
			return nil, errDuplicateInBatch("customer", input.CustomerId)
		}
		seen[input.CustomerId] = true

		customer, err := c.CreateCustomer(ctx, input.CustomerId, input.FirstName,
			input.LastName, input.Email, input.OpeningBalance)
		if err != nil {
			return nil, err
		}
		created = append(created, customer)
	}
	return created, nil
}

func (c *TradeContract) ReadCustomer(
	ctx contractapi.TransactionContextInterface, customerId string,
) (*Customer, error) {
	customerId, err := requireText("customerId", customerId)
	if err != nil {
		return nil, err
	}
	return loadCustomer(ctx, customerId)
}

func (c *TradeContract) GetAllCustomers(
	ctx contractapi.TransactionContextInterface,
) ([]*Customer, error) {
	return readByPrefix[Customer](ctx, prefixCustomer)
}
