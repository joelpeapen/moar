package twin

import "fmt"

// NOTE: This file should do the same thing as internal/panicHandler.go as much
// as possible.

func panicHandler(goroutineName string, recoverResult any, stackTrace []byte) {
	if recoverResult == nil {
		return
	}

	log.Error(fmt.Sprintf("Goroutine panicked: %s: %v\n%s", goroutineName, recoverResult, string(stackTrace)))
}
