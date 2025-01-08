loquet
======

Package loquet (French for "latch") provides a channel with enhanced
broadcast capabilities for Go. Close is now idempotent, and
can broadcast a real value, not just the zero value. 

Okay, okay. Yes, we have to kludge it up and implement 
broadcasting a value in two steps/calls, but you'll 
get the idea from this quick example:

~~~
    closeVal := &Message{}
    myLoquetChan := loquet.NewChan[Message](closeVal)
    myLoquetChan.Close()
	select {
	    case <-myLoquetChan.WhenClosed():
           val, _ := myLoqetChan.Read() // val will be closeVal 
           ...
~~~

The package docs have full details: https://pkg.go.dev/github.com/glycerine/loquet

----
Author: Jason E. Aten, Ph.D.

License: 3-clause BSD style, the same as Go. See LICENSE file.
