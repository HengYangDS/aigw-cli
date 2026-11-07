package invocation

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBindTextTakesOneValue(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
		err  string
	}{
		{[]string{"--label", "Team"}, "Team", ""},
		{[]string{"--label", ""}, "", ""},
		{[]string{"--label", "One", "--label", "Two"}, "", `invalid argument "Two" for "--label" flag: this flag takes one value`},
		{[]string{"--label", "", "--label", "Two"}, "", "this flag takes one value"},
	} {
		var label string
		cmd := &cobra.Command{Use: "demo", RunE: func(*cobra.Command, []string) error { return nil }, SilenceErrors: true, SilenceUsage: true}
		BindText(cmd, &label, "label", "Display name")
		cmd.SetArgs(test.args)
		err := cmd.Execute()
		switch {
		case test.err == "" && (err != nil || label != test.want || !cmd.Flags().Changed("label")):
			t.Errorf("%q: label %q, error %v", test.args, label, err)
		case test.err != "" && (err == nil || !strings.Contains(err.Error(), test.err)):
			t.Errorf("%q: error %v, want %q", test.args, err, test.err)
		}
	}
	// A command line that rebuilds the flag writes its value.
	var label string
	cmd := &cobra.Command{Use: "demo", RunE: func(*cobra.Command, []string) error { return nil }}
	BindText(cmd, &label, "label", "Display name")
	cmd.SetArgs([]string{"--label", "Team"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if line := Rerun(cmd, nil, nil); line != "demo --label Team" {
		t.Fatalf("rebuilt command line = %q", line)
	}
	if usage := cmd.Flags().Lookup("label").Value.Type(); usage != "string" {
		t.Fatalf("type = %q", usage)
	}
}
