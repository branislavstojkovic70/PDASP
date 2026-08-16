#!/usr/bin/env bash
#
# Paying money into merchant and customer accounts.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "06  Deposits"

ORG=org1; USER_ID=org1admin
merchant="MD${RUN_ID}"
app create-merchant "${merchant}" "Deposit Shop ${RUN_ID}" SUPERMARKET "PIB${RUN_ID}" 100 >/dev/null

step "paying into a merchant account"
result=$(app deposit merchant "${merchant}" 250.25)
assert_json "${result}" .oldBalance "100" "the old balance is reported"
assert_json "${result}" .newBalance "350.25" "the new balance is the sum"

ORG=org2; USER_ID=org2user1
customer="CD${RUN_ID}"
app create-customer "${customer}" "Deposit" "Buyer" "dep${RUN_ID}@example.com" 0 >/dev/null

step "paying into a customer account"
result=$(app deposit customer "${customer}" 1000)
assert_json "${result}" .newBalance "1000" "the customer balance grew"

step "amounts are rounded to two decimals after every operation"
app deposit customer "${customer}" 0.1 >/dev/null
app deposit customer "${customer}" 0.2 >/dev/null
balance=$(app customer "${customer}" | jq -r .balance)
assert_numeric_eq "1000.3" "${balance}" "0.1 plus 0.2 does not drift the balance"

summary "06 Deposits"
