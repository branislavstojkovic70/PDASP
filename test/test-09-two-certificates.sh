#!/usr/bin/env bash
#
# The assignment requires the application to work with at least two certificates
# belonging to different organizations.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "09  Two certificates from different organizations"

merchant="MC${RUN_ID}"
customer="CC${RUN_ID}"
product="PC${RUN_ID}"

step "Org1 (Merchants) sets up the merchant and the product"
ORG=org1; USER_ID=org1user1
created=$(app create-merchant "${merchant}" "Cert Shop ${RUN_ID}" SUPERMARKET "PIB${RUN_ID}" 0)
assert_json "${created}" .merchantId "${merchant}" "an Org1MSP identity can write"
app add-product "${merchant}" "${product}" "Cert article" 50 20 >/dev/null

step "Org2 (Customers) registers the customer and buys"
ORG=org2; USER_ID=org2user1
app create-customer "${customer}" "Cert" "Buyer" "cert${RUN_ID}@example.com" 500 >/dev/null
result=$(app buy "${customer}" "${merchant}" "${product}" 2)
assert_json "${result}" .invoice.total "100" "an Org2MSP identity can submit a purchase"

step "Org3 (Regulator) reads what the other two wrote"
ORG=org3; USER_ID=org3user1
assert_json "$(app merchant "${merchant}")" .balance "100" "an Org3MSP identity sees the merchant balance"
assert_json "$(app customer "${customer}")" .balance "400" "and the customer balance"

step "the same data is served by any peer of the organization"
for peer in 0 1 2; do
  balance=$(node "${APP}" merchant "${merchant}" --org org3 --user org3user1 \
    --channel "${CHANNEL}" --peer "${peer}" --compact | jq -r .balance)
  assert_eq "100" "${balance}" "peer${peer}.org3 returns the same state"
done

step "an identity that was never enrolled is refused"
assert_contains "$(node "${APP}" merchants --org org1 --user nobody 2>&1 >/dev/null)" \
  "is not in the wallet" "a missing identity is reported before connecting"

summary "09 Two certificates"
