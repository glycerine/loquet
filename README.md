loquet
======

Package loquet (French for "latch") provides a channel with enhanced
broadcast capabilities for Go. Close is now idempotent, and
can broadcast a real value, not just the zero value.

A loquet.Chan can be re-opened with a call to ReOpen().

----
Author: Jason E. Aten, Ph.D.

License: 3-clause BSD style, the same as Go. See LICENSE file.
