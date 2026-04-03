package ops

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	wslv3 "dialtone/dev/plugins/wsl/src_v3/go"
)

func List(args []string) error {
	fs := flag.NewFlagSet("wsl-list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	instances, err := wslv3.ListInstances()
	if err != nil {
		return err
	}
	if *asJSON {
		raw, err := json.MarshalIndent(instances, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	if len(instances) == 0 {
		fmt.Println("No WSL instances found.")
		return nil
	}
	for _, inst := range instances {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", inst.Name, inst.State, inst.Version, inst.Memory, inst.Disk)
	}
	return nil
}

func Create(args []string) error {
	fs := flag.NewFlagSet("wsl-create", flag.ContinueOnError)
	name := fs.String("name", "", "WSL instance name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" {
		rest := fs.Args()
		if len(rest) > 0 {
			*name = strings.TrimSpace(rest[0])
		}
	}
	return wslv3.CreateInstance(strings.TrimSpace(*name))
}

func Stop(args []string) error {
	fs := flag.NewFlagSet("wsl-stop", flag.ContinueOnError)
	name := fs.String("name", "", "WSL instance name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" {
		rest := fs.Args()
		if len(rest) > 0 {
			*name = strings.TrimSpace(rest[0])
		}
	}
	return wslv3.StopInstance(strings.TrimSpace(*name))
}

func Start(args []string) error {
	fs := flag.NewFlagSet("wsl-start", flag.ContinueOnError)
	name := fs.String("name", "", "WSL instance name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" {
		rest := fs.Args()
		if len(rest) > 0 {
			*name = strings.TrimSpace(rest[0])
		}
	}
	return wslv3.StartInstance(strings.TrimSpace(*name))
}

func Delete(args []string) error {
	fs := flag.NewFlagSet("wsl-delete", flag.ContinueOnError)
	name := fs.String("name", "", "WSL instance name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" {
		rest := fs.Args()
		if len(rest) > 0 {
			*name = strings.TrimSpace(rest[0])
		}
	}
	return wslv3.DeleteInstance(strings.TrimSpace(*name))
}

func Exec(args []string) error {
	fs := flag.NewFlagSet("wsl-exec", flag.ContinueOnError)
	name := fs.String("name", "", "WSL instance name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if strings.TrimSpace(*name) == "" && len(rest) > 0 {
		*name = strings.TrimSpace(rest[0])
		rest = rest[1:]
	}
	out, err := wslv3.ExecInstance(strings.TrimSpace(*name), rest...)
	if out != "" {
		fmt.Println(out)
	}
	return err
}

func OpenTerminal(args []string) error {
	fs := flag.NewFlagSet("wsl-terminal", flag.ContinueOnError)
	name := fs.String("name", "", "WSL instance name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if strings.TrimSpace(*name) == "" && len(rest) > 0 {
		*name = strings.TrimSpace(rest[0])
	}
	return wslv3.OpenTerminal(strings.TrimSpace(*name))
}
