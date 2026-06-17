package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(templateCmd)
	templateCmd.AddCommand(templateListCmd, templateShowCmd, templateEditCmd, templateRmCmd, templatePreviewCmd)
}

var templateCmd = &cobra.Command{Use: "template", Short: "Custom subscription templates per format (dns/rules/proxy-groups)"}

var templateListCmd = &cobra.Command{
	Use: "list", Short: "List formats and whether each has a custom template",
	RunE: func(cmd *cobra.Command, args []string) error {
		custom, err := svc.ListTemplates()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "FORMAT\tCUSTOM\tUPDATED")
		for _, f := range svc.TemplateFormats() {
			ts, ok := custom[f]
			mark, when := "built-in", "-"
			if ok {
				mark, when = "custom", ts.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", f, mark, when)
		}
		return tw.Flush()
	},
}

var templateShowCmd = &cobra.Command{
	Use: "show <format>", Short: "Print the active template (custom, else the built-in starter)", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(templateBody(args[0]))
		return nil
	},
}

var templateEditCmd = &cobra.Command{
	Use: "edit <format>", Short: "Edit a format's template in $EDITOR (creates from a starter)", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := args[0]
		if !contains(svc.TemplateFormats(), format) {
			return fmt.Errorf("unknown format %q (one of: %v)", format, svc.TemplateFormats())
		}
		edited, err := editInEditor(format, templateBody(format))
		if err != nil {
			return err
		}
		if err := svc.SetTemplate(format, edited); err != nil {
			return err
		}
		fmt.Printf("saved template for %s\n", format)
		return nil
	},
}

var templateRmCmd = &cobra.Command{
	Use: "rm <format>", Short: "Remove a custom template (revert to built-in)", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusMsg(svc.DeleteTemplate(args[0]), args[0], "template removed")
	},
}

var templatePreviewCmd = &cobra.Command{
	Use: "preview <format>", Short: "Render the active template against a sample subscription", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := args[0]
		if !contains(svc.TemplateFormats(), format) {
			return fmt.Errorf("unknown format %q (one of: %v)", format, svc.TemplateFormats())
		}
		out, verr := svc.RenderPreview(format, templateBody(format))
		if out == "" && verr != nil {
			return verr // template failed to render at all
		}
		fmt.Println(out)
		if verr != nil {
			fmt.Fprintln(os.Stderr, "⚠ output is not valid "+format+": "+verr.Error())
		}
		return nil
	},
}

// templateBody returns the current custom template, or the starter if none.
func templateBody(format string) string {
	if body, ok := svc.GetTemplate(format); ok {
		return body
	}
	return svc.StarterTemplate(format)
}

// editInEditor writes initial to a temp file, opens $EDITOR, and returns the edited contents.
func editInEditor(format, initial string) (string, error) {
	f, err := os.CreateTemp("", "nexon-"+format+"-*"+templateExt(format))
	if err != nil {
		return "", err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.WriteString(initial); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}
	ed := exec.Command(editor, path)
	ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := ed.Run(); err != nil {
		return "", fmt.Errorf("editor %q failed: %w", editor, err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func templateExt(format string) string {
	if filepath.Ext(format) != "" {
		return ""
	}
	switch format {
	case "clash", "clash-meta":
		return ".yaml"
	default:
		return ".json"
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
