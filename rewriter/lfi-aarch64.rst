Compiler Requirements
+++++++++++++++++++++

When building for the ``aarch64_lfi`` target, the compiler must restrict use of
the instruction set to a subset of instructions, which are known to be safe
from a sandboxing perspective. To do this, we apply a set of simple rewrites at
the assembly language level to transform standard native AArch64 assembly into
LFI-compatible AArch64 assembly.

These rewrites (also called "expansions") are applied at the very end of the
LLVM compilation pipeline (during the assembler step). This allows the rewrites
to be applied to hand-written assembly, including inline assembly.

Compiler Options
================

The LFI target has several configuration options.

* ``+lfi-stores``: create a "stores-only" sandbox, where rewrites are not applied to loads.
* ``+lfi-jumps``: create a "jumps-only" sandbox, where rewrites are not applied to loads/stores.
* ``+lfi-tls-reg``: use an additional reserved register for thread-local storage accesses.

Reserved Registers
==================

The LFI target uses a custom ABI that reserves additional registers for the
platform. The registers are listed below, along with the security invariant
that must be maintained.

* ``x27``: always holds the sandbox base address.
* ``x28``: always holds an address within the sandbox.
* ``sp``: always holds an address within the sandbox.
* ``x30``: always holds an address within the sandbox.
* ``x26``: scratch register.

Assembly Rewrites
=================

Terminology
~~~~~~~~~~~

In the following assembly rewrites, some shorthand is used.

* ``xN`` or ``wN``: refers to any general-purpose non-reserved register.
* ``{a,b,c}``: matches any of ``a``, ``b``, or ``c``.
* ``LDSTr``: a load/store instruction that supports register-register addressing modes, with one source/destination register.
* ``LDSTx``: a load/store instruction not matched by ``LDSTr``.

Control flow
~~~~~~~~~~~~

Indirect branches get rewritten to branch through register ``x28``, which must
always contain an address within the sandbox. An ``add`` is used to safely load
``x28`` with the destination address. Since ``ret`` uses ``x30`` by default,
which already must contain an address within the sandbox, it does not require
any rewrite.

+--------------------+---------------------------+
|      Original      |         Rewritten         |
+--------------------+---------------------------+
| .. code-block::    | .. code-block::           |
|                    |                           |
|    {br,blr,ret} xN |    add x28, x27, wN, uxtw |
|                    |    {br,blr,ret} x28       |
|                    |                           |
+--------------------+---------------------------+
| .. code-block::    | .. code-block::           |
|                    |                           |
|    ret             |    ret                    |
|                    |                           |
+--------------------+---------------------------+

Memory accesses
~~~~~~~~~~~~~~~

Memory accesses are rewritten to use the ``[x27, wM, uxtw]`` addressing mode if
it is available, which is automatically safe. Otherwise, rewrites fall back to
using ``x28`` along with an instruction to safely load it with the target
address.

+---------------------------------+-------------------------------+
|            Original             |           Rewritten           |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTr xN, [xM]               |    LDSTr xN, [x27, wM, uxtw]  |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTr xN, [xM, #I]           |    add x28, x27, wM, uxtw     |
|                                 |    LDSTr xN, [x28, #I]        |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTr xN, [xM, #I]!          |    add xM, xM, #I             |
|                                 |    LDSTr xN, [x27, wM, uxtw]  |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTr xN, [xM], #I           |    LDSTr xN, [x27, wM, uxtw]  |
|                                 |    add xM, xM, #I             |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTr xN, [xM1, xM2]         |    add x26, xM1, xM2          |
|                                 |    LDSTr xN, [x27, w26, uxtw] |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTr xN, [xM1, xM2, MOD #I] |    add x26, xM1, xM2, MOD #I  |
|                                 |    LDSTr xN, [x27, w26, uxtw] |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTx ..., [xM]              |    add x28, x27, wM, uxtw     |
|                                 |    LDSTx ..., [x28]           |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTx ..., [xM, #I]          |    add x28, x27, wM, uxtw     |
|                                 |    LDSTx ..., [x28, #I]       |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTx ..., [xM, #I]!         |    add x28, x27, wM, uxtw     |
|                                 |    LDSTx ..., [x28, #I]       |
|                                 |    add xM, xM, #I             |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTx ..., [xM], #I          |    add x28, x27, wM, uxtw     |
|                                 |    LDSTx ..., [x28]           |
|                                 |    add xM, xM, #I             |
|                                 |                               |
+---------------------------------+-------------------------------+
| .. code-block::                 | .. code-block::               |
|                                 |                               |
|    LDSTx ..., [xM1], xM2        |    add x28, x27, wM1, uxtw    |
|                                 |    LDSTx ..., [x28]           |
|                                 |    add xM1, xM1, xM2          |
|                                 |                               |
+---------------------------------+-------------------------------+

Stack modification
~~~~~~~~~~~~~~~~~~

When the stack pointer is modified, we write the modified value to a temporary,
before loading it back into ``sp`` with a safe ``add``.

+------------------------------+-------------------------------+
|           Original           |           Rewritten           |
+------------------------------+-------------------------------+
| .. code-block::              | .. code-block::               |
|                              |                               |
|    mov sp, xN                |    add sp, x27, wN, uxtw      |
|                              |                               |
+------------------------------+-------------------------------+
| .. code-block::              | .. code-block::               |
|                              |                               |
|    {add,sub} sp, sp, {#I,xN} |    {add,sub} x26, sp, {#I,xN} |
|                              |    add sp, x27, w26, uxtw     |
|                              |                               |
+------------------------------+-------------------------------+

Link register modification
~~~~~~~~~~~~~~~~~~~~~~~~~~~

When the link register is modified, we write the modified value to a
temporary, before loading it back into ``x30`` with a safe ``add``.

+-----------------------+----------------------------+
|       Original        |         Rewritten          |
+-----------------------+----------------------------+
| .. code-block::       | .. code-block::            |
|                       |                            |
|    ldr x30, [...]     |    ldr x26, [...]          |
|                       |    add x30, x27, w26, uxtw |
|                       |                            |
+-----------------------+----------------------------+
| .. code-block::       | .. code-block::            |
|                       |                            |
|    ldp xN, x30, [...] |    ldp xN, x26, [...]      |
|                       |    add x30, x27, w26, uxtw |
|                       |                            |
+-----------------------+----------------------------+
| .. code-block::       | .. code-block::            |
|                       |                            |
|    ldp x30, xN, [...] |    ldp x26, xN, [...]      |
|                       |    add x30, x27, w26, uxtw |
|                       |                            |
+-----------------------+----------------------------+

System instructions
~~~~~~~~~~~~~~~~~~~

System calls are rewritten into a sequence that loads the address of the first
runtime call entrypoint and jumps to it. The runtime call entrypoint table is
stored at the start of the sandbox, so it can be referenced by ``x27``. The
rewrite also saves and restores the link register, since it is used for
branching into the runtime.

+-----------------+----------------------------+
|    Original     |         Rewritten          |
+-----------------+----------------------------+
| .. code-block:: | .. code-block::            |
|                 |                            |
|    svc #0       |    mov w26, w30            |
|                 |    ldr x30, [x27]          |
|                 |    blr x30                 |
|                 |    add x30, x27, w26, uxtw |
|                 |                            |
+-----------------+----------------------------+

Thread-local storage
~~~~~~~~~~~~~~~~~~~~

TLS accesses are rewritten into runtime calls, similar to system calls. The TLS
pointer read runtime call places the TLS pointer value into ``x0``. The TLS
pointer write runtime call writes the value of ``x0`` into the TLS pointer. As
a result, to support reading/writing the TLS pointer using an arbitrary
register ``xN``, the sequences swap ``xN`` and ``x0`` before/after making the
runtime call.

+----------------------+----------------------------+
|       Original       |         Rewritten          |
+----------------------+----------------------------+
| .. code-block::      | .. code-block::            |
|                      |                            |
|    mrs x0, tpidr_el0 |    mov w26, w30            |
|                      |    ldr x30, [x27, #8]      |
|                      |    blr x30                 |
|                      |    add x30, x27, w26, uxtw |
|                      |                            |
+----------------------+----------------------------+
| .. code-block::      | .. code-block::            |
|                      |                            |
|    mrs xN, tpidr_el0 |    mov xN, x0              |
|                      |    mov w26, w30            |
|                      |    ldr x30, [x27, #8]      |
|                      |    blr x30                 |
|                      |    eor x0, x0, xN          |
|                      |    eor xN, x0, xN          |
|                      |    eor x0, x0, xN          |
|                      |    add x30, x27, w26, uxtw |
|                      |                            |
+----------------------+----------------------------+
| .. code-block::      | .. code-block::            |
|                      |                            |
|    msr tpidr_el0, x0 |    mov w26, w30            |
|                      |    ldr x30, [x27, #16]     |
|                      |    blr x30                 |
|                      |    add x30, x27, w26, uxtw |
|                      |                            |
+----------------------+----------------------------+
| .. code-block::      | .. code-block::            |
|                      |                            |
|    msr tpidr_el0, xN |    mov w26, w30            |
|                      |    eor x0, x0, xN          |
|                      |    eor xN, x0, xN          |
|                      |    eor x0, x0, xN          |
|                      |    ldr x30, [x27, #16]     |
|                      |    blr x30                 |
|                      |    eor x0, x0, xN          |
|                      |    eor xN, x0, xN          |
|                      |    eor x0, x0, xN          |
|                      |    add x30, x27, w26, uxtw |
|                      |                            |
+----------------------+----------------------------+

When ``+lfi-tls-reg`` is enabled (experimental), these change to:

+----------------------+-----------------+
|       Original       |    Rewritten    |
+----------------------+-----------------+
| .. code-block::      | .. code-block:: |
|                      |                 |
|    mrs xN, tpidr_el0 |    mov xN, x25  |
|                      |                 |
+----------------------+-----------------+
| .. code-block::      | .. code-block:: |
|                      |                 |
|    mrs tpidr_el0, xN |    mov x25, xN  |
|                      |                 |
+----------------------+-----------------+

Optimizations
=============

Basic guard elimination
~~~~~~~~~~~~~~~~~~~~~~~

If a register is guarded multiple times in the same basic block without any
modifications to it during the intervening instructions, then subsequent guards
can be removed.

+---------------------------+---------------------------+
|         Original          |         Rewritten         |
+---------------------------+---------------------------+
| .. code-block::           | .. code-block::           |
|                           |                           |
|    add x28, x27, wN, uxtw |    add x28, x27, wN, uxtw |
|    ldur xN, [x28]         |    ldur xN, [x28]         |
|    add x28, x27, wN, uxtw |    ldur xN, [x28, #8]     |
|    ldur xN, [x28, #8]     |    ldur xN, [x28, #16]    |
|    add x28, x27, wN, uxtw |                           |
|    ldur xN, [x28, #16]    |                           |
|                           |                           |
+---------------------------+---------------------------+

Stack guard elimination
~~~~~~~~~~~~~~~~~~~~~~~

If the stack pointer is modified by adding/subtracting a small immediate, and
then later used to perform a memory access without any intervening jumps, then
the guard on the stack pointer modification can be removed. This is because the
load/store is guaranteed to trap if the stack pointer has been moved outside of
the sandbox region.

+---------------------------+---------------------------+
|         Original          |         Rewritten         |
+---------------------------+---------------------------+
| .. code-block::           | .. code-block::           |
|                           |                           |
|    add x26, sp, #8        |    add sp, sp, #8         |
|    add sp, x27, w26, uxtw |    ... (same basic block) |
|    ... (same basic block) |    ldr xN, [sp]           |
|    ldr xN, [sp]           |                           |
|                           |                           |
+---------------------------+---------------------------+

Guard hoisting
~~~~~~~~~~~~~~

In certain cases, guards may be hoisted outside of loops.

+-------------------------------+-----------------------+
|           Original            |       Rewritten       |
+-------------------------------+-----------------------+
| .. code-block::               | .. code-block::       |
|                               |                       |
|        mov w8, #10            |        mov w8, #10    |
|        mov w9, #0             |        mov w9, #0     |
|        add x28, x27, wM, uxtw |    .loop:             |
|    .loop:                     |        add w9, w9, #1 |
|        add w9, w9, #1         |        ldr xN, [xM]   |
|        ldr xN, [x28]          |        cmp w9, w8     |
|        cmp w9, w8             |        b.lt .loop     |
|        b.lt .loop             |    .end:              |
|    .end:                      |                       |
|                               |                       |
+-------------------------------+-----------------------+

