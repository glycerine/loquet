loquet
======

Package loquet (French for "latch") provides a channel with enhanced
broadcast capabilities for Go. Close is now idempotent, and
can broadcast a real value, not just the zero value. (Okay, we
have to implement this in two steps/calls, but you'll get the idea.)

The package docs have full details: https://pkg.go.dev/github.com/glycerine/loquet

----
Author: Jason E. Aten, Ph.D.

License: 3-clause BSD style, the same as Go. See LICENSE file.
