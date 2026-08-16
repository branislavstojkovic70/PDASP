#!/usr/bin/env bash
#
# Enrolment and login: the assignment requires the application to work with at
# least two certificates belonging to different organizations.

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_tools
suite "01  Enrolment and identities"

step "enrolling the standard identities of all three organizations"
app_plain bootstrap >/dev/null
assert_eq "0" "$?" "bootstrap enrols without error"

identities=$(app_plain identities)
assert_json "${identities}" 'length >= 9' "true" "wallet holds at least nine identities"

for org in org1 org2 org3; do
  msp=$(echo "${identities}" | jq -r --arg u "${org}user1@${org}" '.[] | select(.label == $u) | .mspId')
  assert_not_empty "${msp}" "${org}user1 is enrolled"
done

step "the three identities map to three different MSPs"
msp1=$(app_plain whoami --org org1 --user org1user1 | jq -r .mspId)
msp2=$(app_plain whoami --org org2 --user org2user1 | jq -r .mspId)
msp3=$(app_plain whoami --org org3 --user org3user1 | jq -r .mspId)
assert_eq "Org1MSP" "${msp1}" "org1user1 belongs to Org1MSP"
assert_eq "Org2MSP" "${msp2}" "org2user1 belongs to Org2MSP"
assert_eq "Org3MSP" "${msp3}" "org3user1 belongs to Org3MSP"

step "NodeOU role is taken from the certificate"
subject=$(app_plain whoami --org org1 --user org1user1 | jq -r .certificateSubject)
assert_contains "${subject}" "OU=client" "the enrolled certificate carries OU=client"

step "registering a brand new identity through the CA"
new_user="clerk${RUN_ID}"
registered=$(app_plain enroll --org org1 --user "${new_user}" --register)
assert_json "${registered}" .registered "true" "a new identity is registered and enrolled"
assert_json "${registered}" .mspId "Org1MSP" "the new identity belongs to Org1MSP"

step "re-registering the same identity must not fail"
app_plain enroll --org org1 --user "${new_user}" --register >/dev/null
assert_eq "0" "$?" "registration is idempotent"

app_plain forget --org org1 --user "${new_user}" >/dev/null

step "an unknown identity produces a clear error"
message=$(node "${APP}" whoami --org org9 --user x 2>&1 >/dev/null)
assert_contains "${message}" "unknown organization" "an unknown organization is rejected"

summary "01 Enrolment"
