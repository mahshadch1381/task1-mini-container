1. Root Filesystem Setup
Accept a folder path as input (e.g. ./rootfs) and treat it as the new root /.
Verify it contains a runnable binary such as /bin/sh.
Use chroot to lock the child process inside that folder.
Change the working directory to / inside the new root.

2. Process Isolation
Start a child process with its own hostname (UTS namespace) and own process list (PID namespace).
Set a custom hostname for it.
Mount /proc inside the new root so tools like ps work.

3. Application Execution
Accept a command to run inside the isolated environment.
Use the exec syscall to replace the child process with that command.
Pass the parent's environment variables through.

4. Cleanup
On exit: unmount /proc and leave nothing mounted behind.

5.A Makefile that builds the program with one command (make).

6.A test.sh script that automatically proves it works by checking:
/bin/sh runs inside the isolated root filesystem,
the hostname inside is different from the host,
/proc is mounted,
processes inside the namespace cannot see the host PID list.
