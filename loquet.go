package loquet

import (
	"sync"
)

// Loquet conveys a value to readers
// via Read(). The value conveyed
// by Read() is called the closeVal.
// Read() can be called before or after
// the Loquet is closed, and will return
// the current closeVal and an isClosed flag
// indicating if the Loquet is closed or not.
//
// The closeVal can be established in three
// different ways:
//
// 1) The closeVal may naturally be set during creation,
// by supplying it to NewLoquet(closeVal).
//
// 2) The closeVal can also be set (or changed) during closing
// with Close(closeVal); Close(nil) is also
// valid but will not alter an established closeVal.
// This simplifies the code that closes the channel:
// it can be unaware of the original/appropriate closeVal.
//
// Close() will close the channel returned
// by WhenClosed(), if it has not already been closed.
// It is safe to call Close() multiple times, knowing
// this will not result in a panic. The underlying
// channel supplied by WhenClosed() is only ever closed
// once.
//
// 3) Lastly, the closeVal can also be updated with
// Set(closeVal). Set will not change the
// closed/open status of the Loquet. If you wish
// to nil out an established closeVal, you must
// use Set(), since Close(nil) does not, on purpose.
// Set on a closed Loquet is valid,
// but there is then no universal guarantee that
// all Read() calls will have returned the
// same closeVal.
//
// A Loquet can only ever go from open to closed. It
// cannot be re-opened once closed. This reflects
// the underlying Go channel semantics.
//
// The closeVal available from Read() is
// independent of the open/closed status of the Loquet,
// and it is up to the user to assign meaning
// to the closeVal, isClosed := Read() values received.
//
// The zero-value of a Loquet is viable, as the
// WhenClosed returned channel is allocated lazily.
// Since it contains a sync.Mutex, a Loquet value
// must not be copied, and should always be
// handled by a pointer.
//
// Notice that the generic parameter is a T but
// all operations deal in *T. For
// example, to work with a closeVal of type *Message,
// simply call NewLoquet[Message](closeVal *Message).
type Loquet[T any] struct {
	mut sync.Mutex

	// ch is closed exactly
	// once on the first Close() call.
	ch chan struct{}

	// closeVal and isClosed are the values that
	// we report from Read().
	closeVal *T
	isClosed bool
}

func NewLoquet[T any](closeVal *T) *Loquet[T] {
	return &Loquet[T]{
		closeVal: closeVal,
	}
}

// Close provides an idempotent close of the
// WhenClosed returned channel. Multiple calls to Close
// will result in only a single close of the
// internal channel provided by WhenClosed().
// This addresses a major design limitation of Go channels.
// By using a Loquet instead of a raw Go channel,
// you need not worry about panic from repeated
// closing.
//
// Close takes an optional (possibly nil) new closeVal
// value to update the current closeVal (from Set or NewLoquet).
//
// Close(nil) is fine too, and a no-op. In this case,
// the internal closeVal will
// not be updated. This avoids the Close() calling
// code needing to know about the appropriate closeVal;
// a frequent case when coordinating goroutine shutdown
// from multiple origins.
//
// Close is also a no-op if the Loquet is already
// closed. The supplied closeVal is then ignored
// and the internal closeVal will not be updated.
//
// If you need to update the internal closeVal
// without closing the Loquet, use Set or SetIfOpen.
func (f *Loquet[T]) Close(closeVal *T) {
	f.mut.Lock()
	defer f.mut.Unlock()

	// ensure only Closed once, and
	// keep only the first close msg,
	// so that transaction style cancel/commit
	// can always defer cancel while letting
	// commit first succeed and be preserved.
	if f.isClosed {
		return
	}
	f.isClosed = true

	// if closeVal is nil, leave f.closeVal as is.
	// i.e. do not over-ride with nil, because closeVal
	// may be already valid (from NewLoqet or Set).
	if closeVal != nil {
		f.closeVal = closeVal
	}
	close(f.ch)
}

// Set changes the closeVal without
// actually closing the Loquet (compare to Close).
// That is, Set will change the closeVal no
// matter the open/closed status
// of the Loquet. Note, however, that if the
// Loquet is already closed, then there is no guarantee that all
// Read()-ers will have received the same closeVal.
// Since this may be a common valid use case, it
// is explicitly allowed (to provide for latch behavior).
// The previously set closeVal is returned in old;
// but this may commonly be ignored.
//
// Use SetIfOpen to set a new closeVal only
// on open Loquets.
func (f *Loquet[T]) Set(closeVal *T) (old *T) {
	f.mut.Lock()
	defer f.mut.Unlock()
	old = f.closeVal
	f.closeVal = closeVal
	return
}

// SetIfOpen is a no-op if the Loquet is closed.
// Otherwise, it behaves like Set().
func (f *Loquet[T]) SetIfOpen(closeVal *T) (old *T) {
	f.mut.Lock()
	defer f.mut.Unlock()
	old = f.closeVal
	if f.isClosed {
		return
	}
	f.closeVal = closeVal
	return
}

// Read returns the current closeVal and the
// isClosed status.
//
// Note that an open channel may supply
// a valid closeVal; but this determined by
// the user. For example, the user may
// have initialized the closeVal during NewLoquet(closeVal),
// or may have subsequently called Set(closeVal).
//
// To avoid busy waiting, use the channal obtained
// by WhenClosed() in your select statements, and
// then call Read() once it is closed.
//
/* For example:
~~~
 ...
 msg := &Message{}
 status := NewLoquet[Message](msg)
 ...
 // to poll the status Loquet:
 // (and because val may be useful even before closing)
 val, isClosed := status.Read()
     ...
 // to wait for the status Loquet to be closed:
 select {
 case <-status.WhenClosed():
	   val, isClosed := status.Read()
      ... react to val... (isClosed will always be true here).
~~~
*/
func (f *Loquet[T]) Read() (closeVal *T, isClosed bool) {
	f.mut.Lock()
	closeVal, isClosed = f.closeVal, f.isClosed
	f.mut.Unlock()
	return
}

// WhenClosed provides a channel that will be
// closed when Loquet.Close() is called. Users
// can then call Loquet.Read() to retreive the current
// closeVal. This two-step process of notification
// then Read is needed because a closed Go
// channel only returns the zero value; Loquet was
// created to work around this limitation.
func (f *Loquet[T]) WhenClosed() <-chan struct{} {
	f.mut.Lock()
	defer f.mut.Unlock()

	// allocated lazily so the zero-value Loquet is viable.
	if f.ch == nil {
		f.ch = make(chan struct{})
	}
	return f.ch
}
