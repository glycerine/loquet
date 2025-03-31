package loquet_test

import (
	"fmt"

	"github.com/glycerine/loquet"
)

type Message struct {
	Err error
}

func doJob(msg *Message, reportBackOn *loquet.Chan[Message]) {

	var err error
	defer reportBackOn.CloseWith(msg)
	//...
	// might return early on error
	if err != nil {
		fmt.Printf("job is erroring out.")
		msg.Err = fmt.Errorf("some error")
		return
	}
	// on success, leave msg.Err == nil
	fmt.Printf("job completed successfully.")
}

// example:
func ExLoquetChanUse() {

	serviceShutdownCh := make(chan struct{})

	msg := &Message{}
	status := loquet.NewChan[Message](msg)

	go doJob(msg, status)

	// poll for status like this.
	closeVal, isClosed := status.Read()
	fmt.Printf("status.Read() returned isClosed=%v, "+
		"closeVal = '%#v'\n", isClosed, closeVal)

	// wait for close like this.
	select {

	case <-status.WhenClosed():
		latest, isClosed := status.Read()
		if !isClosed {
			panic("(should be) impossible, isClosed should be true if WhenClosed() returns! so we can always latest, _ = status.Read() after WhenClosed()")
		}
		fmt.Printf("status.ReadCh returned "+
			"isClosed=%v, latest = '%#v'\n", isClosed, latest)

	case <-serviceShutdownCh:
		fmt.Printf("good: service shutting down, we were not blocked " +
			"permanently on the <-status.Read() above.\n")
		return
	}

}
