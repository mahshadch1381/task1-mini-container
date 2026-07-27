
# Run it with:  sudo ./test.sh
set -u

BINARY=./mahshad-container
NEW_ROOT=./new-root
HOSTNAME_INSIDE=mahshad
CGROUP_ROOT=/sys/fs/cgroup

pass() { echo "  PASS: $1"; }
fail() { echo "  FAIL: $1"; exit 1; }

echo "==> Building..."
go build -o "$BINARY" . || fail "build failed"

# ===========================================================================
# Task 1 tests
# ===========================================================================

# --- Test 1: /bin/sh runs inside the isolated given root
echo "==> Test 1: /bin/sh runs inside the isolated rootfs"
"$BINARY" --rootfs "$NEW_ROOT" /bin/sh -c 'exit 0' || fail "could not run /bin/sh"
pass "/bin/sh ran"

# --- Test 2: hostname is different inside
echo "==> Test 2: hostname is different inside"
host=$(hostname)
inside=$("$BINARY" --rootfs "$NEW_ROOT" --hostname "$HOSTNAME_INSIDE" /bin/sh -c 'hostname' \
	2>/dev/null)
echo "  host=$host  inside=$inside"
{ [ "$inside" = "$HOSTNAME_INSIDE" ] && [ "$inside" != "$host" ]; } || fail "hostname not isolated"
pass "hostname inside is '$inside'"

# --- Test 3: /proc is mounted inside
echo "==> Test 3: /proc is mounted inside"
"$BINARY" --rootfs "$NEW_ROOT" /bin/sh -c '[ -r /proc/1/status ]' || fail "/proc not mounted"
pass "/proc is mounted"

# --- Test 4: PID namespace hides the host process list
echo "==> Test 4: PID namespace hides host processes"
count=$("$BINARY" --rootfs "$NEW_ROOT" /bin/sh -c 'ls -1 /proc | grep -Ec "^[0-9]+$"' \
	2>/dev/null)
echo "  processes visible inside: $count"
[ "$count" -lt 10 ] || fail "too many processes visible; PID namespace not isolated"
pass "only $count processes visible (host list is hidden)"


# Task 2 tests:

# --- Test 5: runs with memory and CPU limits
echo "==> Test 5: runs a process with memory and CPU limits"
out=$("$BINARY" --rootfs "$NEW_ROOT" --memory 100m --cpu 0.5 /bin/sh -c 'echo inside-ok' 2>&1) \
	|| fail "could not run with limits"
echo "$out" | grep -q 'inside-ok' || fail "command output missing"
pass "ran with --memory 100m --cpu 0.5"

# --- Test 6: reporting (pid, cgroup path, exit status) is printed
echo "==> Test 6: runtime reporting is printed"
echo "$out" | grep -q 'pid='            || fail "child PID not printed"
echo "$out" | grep -q 'cgroup created:' || fail "cgroup path not printed"
echo "$out" | grep -q 'container exited:' || fail "exit status not printed"
pass "pid, cgroup path and exit status are reported"

# --- Test 7: the cgroup is created with the requested limits
echo "==> Test 7: cgroup is created with the limits applied"
"$BINARY" --rootfs "$NEW_ROOT" --memory 100m --cpu 0.5 \
	/bin/sh -c 'cat /proc/self/cgroup' >/tmp/mahshad-cg.txt 2>&1 || fail "run failed"
cgpath=$(grep -o "$CGROUP_ROOT/mahshad-[0-9]*" /tmp/mahshad-cg.txt | head -n 1)
[ -n "$cgpath" ] || fail "cgroup path not reported"
echo "  cgroup was: $cgpath"
pass "cgroup $cgpath was created"

# --- Test 8: signals are forwarded to the child
echo "==> Test 8: SIGTERM is forwarded to the child"
"$BINARY" --rootfs "$NEW_ROOT" /bin/sh -c 'sleep 30' >/tmp/mahshad-sig.txt 2>&1 &
runner=$!
sleep 1
kill -TERM "$runner" 2>/dev/null || fail "could not signal the runner"
wait "$runner" 2>/dev/null
grep -q 'forwarding signal' /tmp/mahshad-sig.txt || fail "signal was not forwarded"
grep -q 'container exited:' /tmp/mahshad-sig.txt || fail "exit status not printed after signal"
pass "SIGTERM forwarded and child exited"

# --- Test 9: the cgroup is removed after exit
echo "==> Test 9: cgroup is removed after exit"
[ ! -d "$cgpath" ] || fail "cgroup $cgpath still exists after exit"
leftover=$(ls -d "$CGROUP_ROOT"/mahshad-* 2>/dev/null | wc -l)
[ "$leftover" -eq 0 ] || fail "$leftover leftover cgroup(s) found"
pass "cgroup removed, none left behind"

rm -f /tmp/mahshad-cg.txt /tmp/mahshad-sig.txt

echo ""
echo "ALL TESTS PASSED"
