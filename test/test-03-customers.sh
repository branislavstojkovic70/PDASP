#!/usr/bin/env bash
#
# Registering customers, one at a time and in bulk.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "03  Customers"

# Customers are registered by the Org2 identity, the organization that stands for
# buyers in the mapping described in ARCHITECTURE.md.
ORG=org2; USER_ID=org2user1

step "the initial state holds several customers"
customers=$(app customers)
assert_json "${customers}" 'length >= 4' "true" "at least four customers exist"

step "registering one customer"
customer="CT${RUN_ID}"
created=$(app create-customer "${customer}" "Test" "Customer" "test${RUN_ID}@example.com" 1234.50)
assert_json "${created}" .customerId "${customer}" "the customer is stored"
assert_json "${created}" .balance "1234.5" "the opening balance is recorded"
assert_json "${created}" '.invoices | length' "0" "a new customer has no invoices"

step "reading the customer back"
assert_json "$(app customer "${customer}")" .email "test${RUN_ID}@example.com" "the email reads back"

step "registering several customers at once"
cat > "${TEST_DIR}/fixtures/.customers-${RUN_ID}.json" <<JSON
[
  {"customerId":"CB1${RUN_ID}","firstName":"Bulk","lastName":"One","email":"b1${RUN_ID}@example.com","openingBalance":10},
  {"customerId":"CB2${RUN_ID}","firstName":"Bulk","lastName":"Two","email":"b2${RUN_ID}@example.com","openingBalance":20}
]
JSON
bulk=$(app create-customers --file "${TEST_DIR}/fixtures/.customers-${RUN_ID}.json")
assert_json "${bulk}" 'length' "2" "both customers are created in one transaction"
rm -f "${TEST_DIR}/fixtures/.customers-${RUN_ID}.json"

step "a duplicate id is rejected"
message=$(app_expect_failure create-customer "${customer}" "Other" "Person" "other@example.com" 10)
assert_contains "${message}" "already exists" "a duplicate customer id is refused"

summary "03 Customers"
