package loquet_test

import (
	"fmt"

	"github.com/glycerine/loquet"
)

type Message struct {
	Err error
}

func doJob(msg *Message, reportBackOn *loquet.Loquet[Message]) {

	defer reportBackOn.Close(msg)
	//...
	// might return early on error
	// msg.Err = someErr
	// on success, leave msg.Err == nil
	fmt.Printf("job completed with msg.Err = '%v'\n", msg.Err)
}

func ExampleLoquetUse() {

	serviceShutdownCh := make(chan struct{})

	msg := &Message{}
	status := loquet.NewLoquet[Message](msg)

	go doJob(msg, status)

	closeVal, isClosed := status.Read()
	fmt.Printf("status.Read() returned isClosed=%v, closeVal = '%#v'\n", isClosed, closeVal)

	select {
	//case latest, open = <-status.ReadCh():
	//	fmt.Printf("status.ReadCh returned open=%v, latest = '%#v'", open, latest)
	case <-status.WhenClosed():
		latest, isClosed := status.Read()
		if !isClosed {
			panic("(should be) impossible, isClosed should be true if WhenClosed() returns! so we can always latest, _ = status.Read() after WhenClosed()")
		}
		fmt.Printf("status.ReadCh returned isClosed=%v, latest = '%#v'\n", isClosed, latest)
	case <-serviceShutdownCh:
		fmt.Printf("good: service shutting down, we were not blocked permanently on the <-status.Read() above.\n")
		return
	}

}
