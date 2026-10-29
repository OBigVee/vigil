# Process connections

Press `i` to collect process connections. Review the overview, use Tab for exact JSON, and scroll the left tree with `[` and `]`. Nothing is submitted to an LLM until confirmation.

Linux Unix peer collection queries the kernel's SOCK_DIAG interface for connected peer socket inodes. Vigil resolves visible `/proc/PID/fd` descriptor holders for those inodes. It does not infer peers from matching socket paths. A descriptor can have several holders because of inheritance or sharing; ownership does not prove which process sent a particular message. Socket payloads, file contents, arguments and environment are not read.

Collection is bounded to two seconds, an 8 MiB diagnostic dump, 4096 processes, 2048 descriptors per process, 64 selected connected sockets, and 16 owners per peer socket. Unavailable, partial or truncated collection is noted. Descriptor permissions, process exits, namespace visibility and changing sockets can prevent attribution. No owners found is not proof that no peer exists. Other platforms explicitly report Unix peer collection as unavailable.

Family network collection separately queries the immediate parent (excluding PID 1), visible direct children, then siblings, up to 12 relatives and 16 internet sockets per relative within two seconds. The earlier relationship enumeration caps still apply. Each record identifies its owning PID, process start time and relationship. Parentage does not establish that these processes belong to the same application or communicate. Internet endpoints do not identify remote applications. Existing direct local TCP peer matching remains available.

These are observations over a collection interval, not an atomic snapshot. macOS/Windows use their existing process connection collectors for family activity; native runtime validation is still required for this feature. Unix peer collection is currently Linux-only. Shared memory, named pipes and other IPC mechanisms are not covered.

Kernel interface reference: https://man7.org/linux/man-pages/man7/sock_diag.7.html
