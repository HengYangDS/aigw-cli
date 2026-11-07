package invocation

import (
	"errors"

	"github.com/spf13/cobra"
)

// errValueRepeated refuses a second value for a flag that takes one.
var errValueRepeated = errors.New("this flag takes one value")

// textFlag is the value of a flag that takes one text value, such as a label.
// A second value is refused while the flags are parsed, as a second
// --protocol is, so the last of two values never wins silently. An empty
// value is a value, such as a label that clears.
type textFlag struct {
	target *string
	given  *bool
}

func (flag textFlag) Set(value string) error {
	if *flag.given {
		return errValueRepeated
	}
	*flag.target, *flag.given = value, true
	return nil
}

// String is the value, which a command line that rebuilds the flag writes.
func (flag textFlag) String() string { return *flag.target }

// Type names the value as help names any other text value.
func (textFlag) Type() string { return "string" }

// BindText adds a flag name to cmd that takes one text value, stored in
// target, and refuses a second one.
func BindText(cmd *cobra.Command, target *string, name, usage string) {
	cmd.Flags().Var(textFlag{target: target, given: new(bool)}, name, usage)
}
