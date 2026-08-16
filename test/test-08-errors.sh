#!/usr/bin/env bash
#
# Error handling. The assignment asks explicitly for every error case to be
# covered: missing records, insufficient funds, insufficient stock and invalid
# input.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "08  Error handling"

ORG=org2; USER_ID=org2user1

step "reading records that do not exist"
assert_contains "$(app_expect_failure merchant NOSUCH)" "merchant with id 'NOSUCH' does not exist" \
  "an unknown merchant is reported by id"
assert_contains "$(app_expect_failure customer NOSUCH)" "customer with id 'NOSUCH' does not exist" \
  "an unknown customer is reported by id"
assert_contains "$(app_expect_failure product NOSUCH)" "product with code 'NOSUCH' does not exist" \
  "an unknown product is reported by code"
assert_contains "$(app_expect_failure invoice NOSUCH)" "invoice with id 'NOSUCH' does not exist" \
  "an unknown invoice is reported by id"

step "buying with insufficient funds"
# C004 exists in the initial state with a deliberately small balance.
message=$(app_expect_failure buy C004 M004 P012 1)
assert_contains "${message}" "insufficient funds" "the purchase is refused"
assert_contains "${message}" "C004" "the message names the customer"

step "the refused purchase changed nothing"
before=$(app customer C004 | jq -r .balance)
app_expect_failure buy C004 M004 P012 1 >/dev/null
after=$(app customer C004 | jq -r .balance)
assert_eq "${before}" "${after}" "the balance is untouched"

step "buying more than there is in stock"
assert_contains "$(app_expect_failure buy C003 M004 P012 9999)" "insufficient stock" \
  "the purchase is refused"

step "buying a product from the wrong merchant"
assert_contains "$(app_expect_failure buy C001 M002 P001 1)" "is not offered by merchant" \
  "a product of another merchant is refused"

step "invalid input"
assert_contains "$(app_expect_failure buy C001 M001 P001 0)" "must be greater than zero" \
  "a quantity of zero is refused"
assert_contains "$(app_expect_failure deposit customer C001 -5)" "must be greater than zero" \
  "a negative deposit is refused"
assert_contains "$(app_expect_failure deposit bank C001 5)" "unknown entity type" \
  "an unknown entity type is refused"

ORG=org1; USER_ID=org1admin
assert_contains "$(app_expect_failure create-merchant "MX${RUN_ID}" "Shop" NO_SUCH_TYPE 123 0)" \
  "not in the catalogue" "an unknown merchant type is refused"
assert_contains "$(app_expect_failure create-customer "CX${RUN_ID}" A B not-an-email 0)" \
  "must contain @" "an invalid email is refused"
assert_contains "$(app_expect_failure add-product M001 "PX${RUN_ID}" Item 100 5 31.12.2026)" \
  "YYYY-MM-DD" "an invalid expiry date is refused"
assert_contains "$(app_expect_failure add-product M001 "PY${RUN_ID}" Item -10 5)" \
  "must be greater than zero" "a negative price is refused"

step "usage errors are caught before any network call"
assert_contains "$(app_expect_failure buy C001)" "missing argument" "a missing argument is reported"
assert_contains "$(node "${APP}" no-such-command 2>&1 >/dev/null)" "unknown command" \
  "an unknown command is reported"

summary "08 Error handling"
