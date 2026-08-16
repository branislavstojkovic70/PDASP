#!/usr/bin/env bash
#
# Both channels carry the same chaincode but separate ledgers.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools; require_identities
suite "10  Two channels"

ORG=org1; USER_ID=org1admin
merchant="MH${RUN_ID}"

step "the chaincode answers on both channels"
CHANNEL=channel1
assert_json "$(app merchants)" 'length >= 4' "true" "channel1 holds the initial state"
CHANNEL=channel2
assert_json "$(app merchants)" 'length >= 4' "true" "channel2 holds the initial state"

step "a record written on channel1 is invisible on channel2"
CHANNEL=channel1
app create-merchant "${merchant}" "Channel Shop ${RUN_ID}" SUPERMARKET "PIB${RUN_ID}" 42 >/dev/null
assert_json "$(app merchant "${merchant}")" .balance "42" "the merchant exists on channel1"

CHANNEL=channel2
assert_contains "$(app_expect_failure merchant "${merchant}")" "does not exist" \
  "the merchant does not exist on channel2"

step "the same id can be reused on the other channel with different data"
app create-merchant "${merchant}" "Other Channel Shop" SUPERMARKET "PIB2${RUN_ID}" 99 >/dev/null
assert_json "$(app merchant "${merchant}")" .balance "99" "channel2 has its own record under the same id"

CHANNEL=channel1
assert_json "$(app merchant "${merchant}")" .balance "42" "channel1 is unaffected"

step "an unknown channel is rejected before connecting"
assert_contains "$(node "${APP}" merchants --channel channel9 2>&1 >/dev/null)" "unknown channel" \
  "a non existent channel is reported"

summary "10 Two channels"
