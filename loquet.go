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
		ch:       make(chan struct{}),
	}
}

// The Close() call can provide
// an optional new closeVal value to over-rider
// the closeVal set in NewLoquet(), or
// Close(nil) is fine: the original NewLoquet(closeVal)
// will be supplied on Read() in that case. This
// avoids the Close() calling code needing to
// know about the appropriate closeVal.
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
// actually closing the Loquet. It will change
// the closeVal no matter the open/closed status
// of the Loquet. Note, however, that if the
// Loquet is already closed, then there is no guarantee that all
// Read()-ers will have received the same closeVal.
// Since this may be a common valid use case, it
// is explicitly allowed (to provide for latch behavior).
// The previously set closeVal is returned in old;
// but this may commonly be ignored.
func (f *Loquet[T]) Set(closeVal *T) (old *T) {
	f.mut.Lock()
	defer f.mut.Unlock()
	old = f.closeVal
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
// ...
// msg := &Message{}
// status := NewLoquet[Message](msg)
// ...
// // to poll the status Loquet:
// // (and because val may be useful even before closing)
// val, isClosed := status.Read()
//     ...
// // to wait for the status Loquet to be closed:
// select {
// case <-status.WhenClosed():
//	   val, isClosed := status.Read()
//	   ... react to val... (isClosed will always be true here).
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

	return f.ch
}
