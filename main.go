package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/RedHatInsights/host-metering/config"
	"github.com/RedHatInsights/host-metering/daemon"
	"github.com/RedHatInsights/host-metering/logger"
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
		var logMessages strings.Builder

		logMessages.WriteString("Updating config from config file...\n")
		err := cfg.UpdateFromConfigFile(*configPath)
		if err != nil {
			logMessages.WriteString(fmt.Sprintf("Failed to process file: %v\n", err.Error()))
		}

		logMessages.WriteString("Updating config from environment variables...\n")
		err = cfg.UpdateFromEnvVars()
		if err != nil {
			logMessages.WriteString(fmt.Sprintf("Failed to process variables: %v\n", err.Error()))
		}

		// initialize the logger according to the given configuration
		err = logger.InitLogger(cfg.LogPath, cfg.LogLevel)

		if err != nil {
			logger.Debugf("Error initializing logger: %s\n", err.Error())
		}

		//Now that the logger is configured, we can report configuration state.
		logger.Infoln(logMessages.String())

		//print out the configuration
		logger.Infoln(cfg.String())

		cv := config.NewConfigValidator(cfg)
		err = cv.Validate()
		if err != nil {
			logger.Errorf("Invalid configuration: %v\n", err.Error())
			os.Exit(2)
		}

		d, err := daemon.NewDaemon(cfg)
		if err != nil {
			logger.Errorf("Failed to create daemon: %v\n", err.Error())
			os.Exit(1)
		}

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
	fmt.Println("Usage: host-metering [OPTIONS] SUBCOMMAND")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("Subcommands:")
	fmt.Println("  daemon    Run in daemon mode")
	fmt.Println("  once      Execute once")
	fmt.Println("  help      Print this help message")
}
