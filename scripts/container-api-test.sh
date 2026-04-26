#!/bin/sh
set -e
# Run inside freerange-notification-service container; /tmp/p.json = login body; outputs steps to stdout
LOGIN=$(wget -qO- -T 25 --header="Content-Type: application/json" --post-file=/tmp/p.json http://127.0.0.1:8080/v1/auth/login)
TOKEN=$(echo "$LOGIN" | tr -d '\n' | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
if [ -z "$TOKEN" ]; then echo "LOGIN_FAIL"; echo "$LOGIN"; exit 1; fi
export TOKEN
APPS=$(wget -qO- -T 25 --header="Authorization: Bearer $TOKEN" http://127.0.0.1:8080/v1/apps/)
# naive extract first api_key (works for our JSON shape)
APIKEY=$(echo "$APPS" | tr -d '\n' | sed -n 's/.*"api_key":"\([^"]*\)".*/\1/p' | head -1)
if [ -z "$APIKEY" ]; then echo "APPS_FAIL"; echo "$APPS"; exit 1; fi
EM="wa-e2e-$(date +%s)@example.com"
USERBODY=$(printf '{"email":"%s","phone":"+918969598267","full_name":"Container E2E"}' "$EM")
printf '%s' "$USERBODY" > /tmp/ub.json
wget -qO- -T 25 --header="Content-Type: application/json" --header="X-API-Key: $APIKEY" --header="Authorization: Bearer $TOKEN" --post-file=/tmp/ub.json http://127.0.0.1:8080/v1/users/ > /tmp/uc.json || true
QSBODY=$(printf '{"to":"%s","channel":"whatsapp","subject":"Test","body":"FreeRange container test %s"}' "$EM" "$(date -Iseconds 2>/dev/null || date)")
printf '%s' "$QSBODY" > /tmp/qs.json
SEND=$(wget -qO- -T 40 --header="Content-Type: application/json" --header="X-API-Key: $APIKEY" --header="Authorization: Bearer $TOKEN" --post-file=/tmp/qs.json http://127.0.0.1:8080/v1/quick-send)
echo "QUICK_SEND_RESULT=$SEND"
