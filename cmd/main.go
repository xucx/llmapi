package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/xucx/llmapi/internal/server"
	"github.com/xucx/llmapi/internal/version"
	"github.com/xucx/llmapi/log"

	"github.com/spf13/cobra"
)

var (
	configFile string
	cmd        = &cobra.Command{
		Use:           "llmapi",
		Short:         "llmapi",
		Version:       version.Version + " " + version.BuildRevision + " " + version.BuildTimestamp,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cmd.Context())
		},
	}
)

func runServer(ctx context.Context) error {

	if configFile != "" {
		if err := server.LoadConfig(configFile); err != nil {
			return fmt.Errorf("load config file %s fail: %v", configFile, err)
		}
	}

	log.SetLogger(log.WarpZapLogger(log.NewZapLogger(server.C.Log)))

	return server.Run(ctx)
}

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	cobra.CheckErr(cmd.ExecuteContext(ctx))
}

func init() {
	flags := cmd.Flags()
	flags.StringVarP(&configFile, "config", "c", "", "config file")

}
