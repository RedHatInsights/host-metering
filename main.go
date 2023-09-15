package main

import (
	"flag"
	"fmt"
	"strings"

	"redhat.com/milton/config"
	"redhat.com/milton/daemon"
	"redhat.com/milton/logger"
)

func main() {
	configPath := flag.String("config", config.DefaultConfigPath, "Configuration file path")

	flag.NewFlagSet("help", flag.ExitOnError)
	flag.NewFlagSet("daemon", flag.ExitOnError)
	flag.NewFlagSet("once", flag.ExitOnError)
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Error: no subcommand specified")
		printUsage()
		return
	}

	command := args[0]
	switch command {
	case "help":
		printUsage()
	case "daemon", "once":
		cfg := config.NewConfig()

		var configurationErrors strings.Builder

		configurationErrors.WriteString("Updating config from config file...\n")
		errors := cfg.UpdateFromConfigFile(*configPath)
		if errors != "" {
			configurationErrors.WriteString(errors)
		}

		configurationErrors.WriteString("Updating config from environment variables...\n")
		errors = cfg.UpdateFromEnvVars()
		if errors != "" {
			configurationErrors.WriteString(errors)
		}

		// initialize the logger according to the given configuration
		err := logger.InitLogger(cfg.LogPath, cfg.LogLevel)

		if err != nil {
			logger.Debugf("Error initializing logger: %s\n", err.Error())
		}

		//Now that the logger is configured, we can report configuration state.
		logger.Debugln(configurationErrors.String())

		//print out the configuration
		logger.Infoln(cfg.String())

		d := daemon.NewDaemon(cfg)

		if command == "once" {
			d.RunOnce()
			return
		}
		d.Run()
	default:
		fmt.Println("Error: unknown subcommand", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: milton [OPTIONS] SUBCOMMAND")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("Subcommands:")
	fmt.Println("  daemon    Run in daemon mode")
	fmt.Println("  once      Execute once")
	fmt.Println("  help      Print this help message")
}
