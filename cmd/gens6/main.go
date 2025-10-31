package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"mko.re/gens6/api/gens6pb"
	"mko.re/go/errors"
)

var colors = []string{
	"31",
	"32",
	"33",
	"34",
	"35",
	"36",
	"37",
	"90",
	"91",
	"92",
	"93",
	"94",
	"95",
	"96",
	"97",
}

func run() error {
	outputDir := flag.String("output", "", "output directory")
	configFile := flag.String("config", "", "config file")
	flag.Parse()

	if *outputDir == "" {
		return errors.New("--output flag is required")
	}
	if *configFile == "" {
		return errors.New("--config flag is required")
	}

	configData, err := os.ReadFile(*configFile)
	if err != nil {
		return errors.WithStack(err)
	}
	config := &gens6pb.Config{}
	if err := prototext.Unmarshal(configData, config); err != nil {
		return errors.WithStack(err)
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		return errors.WithStack(err)
	}

	scanDir := filepath.Join(*outputDir, "_scan")
	if err := os.RemoveAll(scanDir); err != nil {
		return errors.WithStack(err)
	}
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		return errors.WithStack(err)
	}

	for i, service := range config.GetServices() {
		serviceDir := filepath.Join(*outputDir, service.GetName())

		if err := os.MkdirAll(serviceDir, 0755); err != nil {
			return errors.WithStack(err)
		}

		////////////////////////////////////////////////////////////////////////////////
		// Run script
		////////////////////////////////////////////////////////////////////////////////

		runFile := filepath.Join(serviceDir, "run")
		var sb strings.Builder
		sb.WriteString("#!/usr/bin/env execlineb\n\n")
		sb.WriteString("fdmove -c 2 1\n")
		for _, dep := range service.GetDependencies() {
			sb.WriteString(fmt.Sprintf("foreground { echo \"↗ waiting for %s…\" }\n", dep))
			sb.WriteString(fmt.Sprintf("foreground { s6-svwait -U ../%s }\n", dep))
		}
		if service.GetReady().GetFd().GetEnv() != "" {
			sb.WriteString(fmt.Sprintf("export %s 3\n", service.GetReady().GetFd().GetEnv()))
		}
		sb.WriteString("foreground { echo \"↗ starting…\" }\n")
		sb.WriteString(script(service.GetRun()))
		if err := os.WriteFile(runFile, []byte(sb.String()), 0755); err != nil {
			return errors.WithStack(err)
		}

		////////////////////////////////////////////////////////////////////////////////
		// Log script
		////////////////////////////////////////////////////////////////////////////////

		logDir := filepath.Join(serviceDir, "log")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return errors.WithStack(err)
		}
		logRunFile := filepath.Join(logDir, "run")
		colorstart := fmt.Sprintf("\033[%sm", colors[i%len(colors)])
		colorend := "\033[0m"
		s6logScript := fmt.Sprintf(`#!/usr/bin/env execlineb
foreground { rm -rf ./log }
s6-log T n1 ./log T p"%s[%s]%s" 1
`, colorstart, service.GetName(), colorend)
		if err := os.WriteFile(logRunFile, []byte(s6logScript), 0755); err != nil {
			return errors.WithStack(err)
		}

		////////////////////////////////////////////////////////////////////////////////
		// Finish script
		////////////////////////////////////////////////////////////////////////////////

		finishFile := filepath.Join(serviceDir, "finish")
		sb = strings.Builder{}
		sb.WriteString("#!/usr/bin/env execlineb\n\n")
		sb.WriteString("foreground { echo \"↘ stopped\" }\n")
		sb.WriteString(script(service.GetFinish()))
		if err := os.WriteFile(finishFile, []byte(sb.String()), 0755); err != nil {
			return errors.WithStack(err)
		}

		////////////////////////////////////////////////////////////////////////////////
		// Ready notification
		////////////////////////////////////////////////////////////////////////////////

		notificationFDFile := filepath.Join(serviceDir, "notification-fd")
		if service.GetReady().GetFd() != nil {
			if err := os.WriteFile(notificationFDFile, []byte("3"), 0600); err != nil {
				return errors.WithStack(err)
			}
		} else if err := os.Remove(notificationFDFile); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		////////////////////////////////////////////////////////////////////////////////
		// Down
		////////////////////////////////////////////////////////////////////////////////

		downFile := filepath.Join(serviceDir, "down")
		if service.GetDown() {
			if err := os.WriteFile(downFile, []byte{}, 0600); err != nil {
				return errors.WithStack(err)
			}
		} else if err := os.Remove(downFile); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		////////////////////////////////////////////////////////////////////////////////
		// Scan dir entry
		////////////////////////////////////////////////////////////////////////////////

		if err := os.Symlink("../"+service.GetName(), filepath.Join(scanDir, service.GetName())); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func script(s string) string {
	if !strings.HasPrefix(s, "#!") {
		return s
	}
	ss := strings.SplitN(s, "\n", 2)
	interpreter := strings.TrimSpace(ss[0][2:])
	body := ""
	if len(ss) > 1 {
		body = ss[1]
	}
	return fmt.Sprintf("heredoc 0 \"%s\" %s", escape(body), interpreter)
}

func escape(s string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(s, "\\", "\\\\"),
		"\"", "\\\"")
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
