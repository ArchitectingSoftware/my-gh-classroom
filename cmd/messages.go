package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/ArchitectingSoftware/my-gh-classroom/internal/course"
)

// messagesFile is where --message output is saved. It is overwritten on
// every run and contains names, so it should stay out of version control.
var messagesFile = "messages.txt"

// emitMessages prints ready-to-paste messages and saves them to
// messagesFile. mgc never sends them.
func emitMessages(msgs []course.Message, what string) error {
	if len(msgs) == 0 {
		// Overwrite any earlier file so stale messages are never pasted.
		note := fmt.Sprintf("No messages to write: %s.\n", what)
		fmt.Print("\n" + note)
		if err := os.WriteFile(messagesFile, []byte(note), 0o600); err != nil {
			return fmt.Errorf("could not write %s: %w", messagesFile, err)
		}
		return nil
	}
	var buf bytes.Buffer
	course.WriteMessages(&buf, msgs)
	fmt.Printf("\nMessages (%d), ready to paste into email or Canvas:\n\n", len(msgs))
	os.Stdout.Write(buf.Bytes())
	if err := os.WriteFile(messagesFile, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("could not write %s: %w", messagesFile, err)
	}
	fmt.Printf("\nMessages also saved to %s\n", messagesFile)
	return nil
}
