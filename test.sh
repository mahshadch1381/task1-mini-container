
# Run it with:  sudo ./test.sh
set -u

BINARY=./mahshad-container
NEW_ROOT=./new-root
HOSTNAME_INSIDE=mahshad

pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1"; exit 1; }

echo "==> Building..."
go build -o "$BINARY" . || fail "build failed"

# --- Test 1: /bin/sh runs inside the isolated given root
echo "==> Test 1: /bin/sh runs inside the isolated rootfs"
"$BINARY" --rootfs "$NEW_ROOT" /bin/sh -c 'exit 0' || fail "could not run /bin/sh"
pass "/bin/sh ran"

# --- Test 2: hostname is different inside 
echo "==> Test 2: hostname is different inside"
host=$(hostname)
inside=$("$BINARY" --rootfs "$NEW_ROOT" --hostname "$HOSTNAME_INSIDE" /bin/sh -c 'hostname')
echo "  host=$host  inside=$inside"
{ [ "$inside" = "$HOSTNAME_INSIDE" ] && [ "$inside" != "$host" ]; } || fail "hostname not isolated"
pass "hostname inside is '$inside'"

# --- Test 3: /proc is mounted inside 
echo "==> Test 3: /proc is mounted inside"
"$BINARY" --rootfs "$NEW_ROOT" /bin/sh -c '[ -r /proc/1/status ]' || fail "/proc not mounted"
pass "/proc is mounted"

# --- Test 4: PID namespace hides the host process list
echo "==> Test 4: PID namespace hides host processes"
count=$("$BINARY" --rootfs "$NEW_ROOT" /bin/sh -c 'ls -1 /proc | grep -Ec "^[0-9]+$"')
echo "  processes visible inside: $count"
[ "$count" -lt 10 ] || fail "too many processes visible; PID namespace not isolated"
pass "only $count processes visible (host list is hidden)"

echo ""
echo "ALL TESTS PASSED"
