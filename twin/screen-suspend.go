//go:build !windows

package twin

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Suspend and wait for SIGCONT, then resume. Basically ctrl-Z handling.
//
// So this method will not return until after the process is resumed again.
func (screen *UnixScreen) suspend() error {
	cont := make(chan os.Signal, 1)
	signal.Notify(cont, syscall.SIGCONT)
	defer signal.Stop(cont)

	return screen.PauseAndCall(func() error {
		// kill(0) = "Send signal to all processes in the current process group"
		err := syscall.Kill(0, syscall.SIGTSTP)
		if err != nil {
			return fmt.Errorf("failed to suspend process group: %w", err)
		}

		// Wait for SIGCONT signal to arrive
		<-cont

		return nil
	})
}
