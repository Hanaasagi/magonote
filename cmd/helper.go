// nolint:errcheck
package cmd

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	titleStyle       = color.New(color.Bold, color.FgHiWhite)
	commandStyle     = color.New(color.FgHiGreen)
	descriptionStyle = color.New(color.FgHiCyan)
	aliasStyle       = color.New(color.FgHiGreen)
	exampleStyle     = color.New(color.FgHiCyan)
	flagStyle        = color.New(color.Bold, color.FgHiCyan)
	tipStyle         = color.New(color.FgHiYellow)
	groupTitleStyle  = color.New(color.Bold, color.FgHiMagenta)
)

const UsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

var HelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}` + titleStyle.Sprintf("GitHub:") + color.New(color.FgYellow).Sprintln(
	"		https://github.com/Hanaasagi/magonote",
)

func rpad(s string, padding int) string {
	template := fmt.Sprintf("%%-%ds", padding)
	return fmt.Sprintf(template, s)
}

func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

func allChildCommandsHaveGroup(cmd *cobra.Command) bool {
	for _, subcmd := range cmd.Commands() {
		if subcmd.GroupID == "" && subcmd.IsAvailableCommand() {
			return false
		}
	}
	return true
}

var (
	reWithShort = regexp.MustCompile(`^( {2,})(-[a-zA-Z]), (--[a-zA-Z0-9-]+)(.*)$`)
	reLongOnly  = regexp.MustCompile(`^( {2,})(--[a-zA-Z0-9-]+)(.*)$`)
)

func colorFlags(raw string) []byte {
	lines := strings.Split(raw, "\n")

	var out bytes.Buffer

	for _, line := range lines {
		switch {
		case reWithShort.MatchString(line):
			m := reWithShort.FindStringSubmatch(line)
			indent, shortFlag, longFlag, rest := m[1], m[2], m[3], m[4]
			out.WriteString(indent)
			flagStyle.Fprint(&out, shortFlag)
			out.WriteString(", ")
			// flagStyle.Fprint(&out, longFlag)
			out.WriteString(longFlag)
			out.WriteString(rest)
			out.WriteByte('\n')

		case reLongOnly.MatchString(line):
			m := reLongOnly.FindStringSubmatch(line)
			indent, longFlag, rest := m[1], m[2], m[3]
			out.WriteString(indent)
			flagStyle.Fprint(&out, longFlag)
			out.WriteString(rest)
			out.WriteByte('\n')

		default:
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}

	return out.Bytes()

}

func ColorUsageFunc(w io.Writer, cmd *cobra.Command) error {
	buf := &bytes.Buffer{}

	titleStyle.Fprint(buf, "Usage:")
	if cmd.Runnable() {
		fmt.Fprint(buf, "\n  ")
		commandStyle.Fprint(buf, cmd.UseLine())
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprint(buf, "\n  ")
		commandStyle.Fprintf(buf, "%s [command]", cmd.CommandPath())
	}

	if len(cmd.Aliases) > 0 {
		fmt.Fprint(buf, "\n\n")
		titleStyle.Fprint(buf, "Aliases:")
		fmt.Fprint(buf, "\n  ")
		aliasStyle.Fprint(buf, strings.Join(cmd.Aliases, ", "))
	}

	if cmd.HasExample() {
		fmt.Fprint(buf, "\n\n")
		titleStyle.Fprint(buf, "Examples:")
		fmt.Fprint(buf, "\n")
		exampleStyle.Fprint(buf, cmd.Example)
	}

	if cmd.HasAvailableSubCommands() {
		cmds := cmd.Commands()

		if len(cmd.Groups()) == 0 {
			fmt.Fprint(buf, "\n\n")
			titleStyle.Fprint(buf, "Available Commands:")
			for _, subcmd := range cmds {
				if subcmd.IsAvailableCommand() || subcmd.Name() == "help" {
					fmt.Fprint(buf, "\n  ")
					commandStyle.Fprint(buf, rpad(subcmd.Name(), subcmd.NamePadding()))
					fmt.Fprint(buf, " ")
					descriptionStyle.Fprint(buf, subcmd.Short)
				}
			}
		} else {
			for _, group := range cmd.Groups() {
				fmt.Fprint(buf, "\n\n")
				groupTitleStyle.Fprint(buf, group.Title)
				for _, subcmd := range cmds {
					if subcmd.GroupID == group.ID && (subcmd.IsAvailableCommand() || subcmd.Name() == "help") {
						fmt.Fprint(buf, "\n  ")
						commandStyle.Fprint(buf, rpad(subcmd.Name(), subcmd.NamePadding()))
						fmt.Fprint(buf, " ")
						descriptionStyle.Fprint(buf, subcmd.Short)
					}
				}
			}

			if !allChildCommandsHaveGroup(cmd) {
				fmt.Fprint(buf, "\n\n")
				titleStyle.Fprint(buf, "Additional Commands:")
				for _, subcmd := range cmds {
					if subcmd.GroupID == "" && (subcmd.IsAvailableCommand() || subcmd.Name() == "help") {
						fmt.Fprint(buf, "\n  ")
						commandStyle.Fprint(buf, rpad(subcmd.Name(), subcmd.NamePadding()))
						fmt.Fprint(buf, " ")
						descriptionStyle.Fprint(buf, subcmd.Short)
					}
				}
			}
		}
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Fprint(buf, "\n\n")
		titleStyle.Fprint(buf, "Flags:")
		fmt.Fprint(buf, "\n")

		raw := trimRightSpace(cmd.LocalFlags().FlagUsages())
		buf.Write(colorFlags(raw))
	}

	if cmd.HasAvailableInheritedFlags() {
		fmt.Fprint(buf, "\n\n")
		flagStyle.Fprint(buf, "Global Flags:")
		fmt.Fprint(buf, "\n")
		fmt.Fprint(buf, trimRightSpace(cmd.InheritedFlags().FlagUsages()))
	}

	if cmd.HasHelpSubCommands() {
		fmt.Fprint(buf, "\n\n")
		titleStyle.Fprint(buf, "Additional help topics:")
		for _, subcmd := range cmd.Commands() {
			if subcmd.IsAdditionalHelpTopicCommand() {
				fmt.Fprint(buf, "\n  ")
				commandStyle.Fprint(buf, rpad(subcmd.CommandPath(), subcmd.CommandPathPadding()))
				fmt.Fprint(buf, " ")
				descriptionStyle.Fprint(buf, subcmd.Short)
			}
		}
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Fprint(buf, "\n\n")
		tipStyle.Fprintf(buf, "Use \"%s [command] --help\" for more information about a command.", cmd.CommandPath())
	}

	fmt.Fprintln(buf)

	_, err := w.Write(buf.Bytes())
	return err
}

func ColorHelpFunc(c *cobra.Command, _ []string) {
	ColorUsageFunc(c.OutOrStdout(), c)
}
