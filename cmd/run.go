package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"myproxy/config"
	"myproxy/internal"
	"myproxy/internal/mlog"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
)

const (
	configPath = "config"
)

func init() {
	rootCmd.Flags().StringP(configPath, "c", "config.yaml", "path for config file")
}

var rootCmd = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		return execute()
	},
}

func Run() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(time.Now().Format(time.RFC3339), "	ERROR	", err)
		os.Exit(1)
	}
}

func execute() error {
	cPath, err := rootCmd.Flags().GetString(configPath)
	if err != nil {
		return err
	}

	c, err := config.Init(cPath)
	if err != nil {
		return err
	}

	instance, err := internal.New(c)
	if err != nil {
		return err
	}

	if err = instance.Start(); err != nil {
		return errors.New("Failed to start: " + err.Error())
	}
	defer func(instance *internal.Instance) {
		err := instance.Close()
		if err != nil {
			mlog.Error("", zap.Error(err))
		}
	}(instance)

	runtime.GC()
	debug.FreeOSMemory()

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
		<-osSignals
	}
	return nil
}
