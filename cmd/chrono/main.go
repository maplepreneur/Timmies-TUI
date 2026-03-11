package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/maplepreneur/chrono/internal/export"
	"github.com/maplepreneur/chrono/internal/report"
	"github.com/maplepreneur/chrono/internal/service"
	sqlstore "github.com/maplepreneur/chrono/internal/store/sqlite"
	"github.com/maplepreneur/chrono/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var dbPath string
	root := &cobra.Command{Use: "chrono", Short: "Chrono is a CLI/TUI time tracker"}
	root.PersistentFlags().StringVar(&dbPath, "db", "chrono.db", "sqlite database path")

	withDeps := func(run func(*sqlstore.Store, *service.TimerService, *cobra.Command, []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			store, err := sqlstore.Open(dbPath)
			if err != nil {
				return err
			}
			defer store.Close()
			svc := service.NewTimerService(store)
			return run(store, svc, cmd, args)
		}
	}

	root.AddCommand(&cobra.Command{
		Use:   "tui",
		Short: "Launch terminal UI",
		RunE: withDeps(func(store *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			return tui.Run(store, svc)
		}),
	})

	clientCmd := &cobra.Command{Use: "client", Short: "Manage clients"}
	clientAddCmd := &cobra.Command{
		Use:   "add [name]",
		Args:  cobra.ExactArgs(1),
		Short: "Add a client",
		RunE: withDeps(func(store *sqlstore.Store, _ *service.TimerService, cmd *cobra.Command, args []string) error {
			if err := store.AddClient(args[0]); err != nil {
				return err
			}
			fmt.Printf("added client: %s\n", args[0])
			return nil
		}),
	}
	clientListCmd := &cobra.Command{
		Use:   "list",
		Short: "List clients",
		RunE: withDeps(func(store *sqlstore.Store, _ *service.TimerService, cmd *cobra.Command, args []string) error {
			clients, err := store.ListClients()
			if err != nil {
				return err
			}
			for _, c := range clients {
				fmt.Printf("- %s\n", c)
			}
			return nil
		}),
	}
	clientCmd.AddCommand(clientAddCmd, clientListCmd)
	root.AddCommand(clientCmd)

	typeCmd := &cobra.Command{Use: "type", Short: "Manage tracking types"}
	typeAddCmd := &cobra.Command{
		Use:   "add [name]",
		Args:  cobra.ExactArgs(1),
		Short: "Add a tracking type",
		RunE: withDeps(func(store *sqlstore.Store, _ *service.TimerService, cmd *cobra.Command, args []string) error {
			if err := store.AddTrackingType(args[0]); err != nil {
				return err
			}
			fmt.Printf("added type: %s\n", args[0])
			return nil
		}),
	}
	typeListCmd := &cobra.Command{
		Use:   "list",
		Short: "List tracking types",
		RunE: withDeps(func(store *sqlstore.Store, _ *service.TimerService, cmd *cobra.Command, args []string) error {
			types, err := store.ListTrackingTypes()
			if err != nil {
				return err
			}
			for _, t := range types {
				fmt.Printf("- %s\n", t)
			}
			return nil
		}),
	}
	typeCmd.AddCommand(typeAddCmd, typeListCmd)
	root.AddCommand(typeCmd)

	var clientName, trackingTypeName, note string
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new timer",
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if clientName == "" || trackingTypeName == "" {
				return errors.New("--client and --type are required")
			}
			if _, err := svc.Start(clientName, trackingTypeName, note); err != nil {
				return err
			}
			fmt.Printf("started session for @%s (%s)\n", clientName, trackingTypeName)
			return nil
		}),
	}
	startCmd.Flags().StringVar(&clientName, "client", "", "client name (without @)")
	startCmd.Flags().StringVar(&trackingTypeName, "type", "", "tracking type name")
	startCmd.Flags().StringVar(&note, "note", "", "session note")
	root.AddCommand(startCmd)

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the active timer",
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if _, err := svc.Stop(); err != nil {
				return err
			}
			fmt.Println("stopped active session")
			return nil
		}),
	}
	root.AddCommand(stopCmd)

	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume latest stopped session as a new segment",
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if _, err := svc.Resume(); err != nil {
				return err
			}
			fmt.Println("resumed latest stopped session")
			return nil
		}),
	}
	root.AddCommand(resumeCmd)

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show active timer status",
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			active, err := svc.Status()
			if err != nil {
				return err
			}
			if active == nil {
				fmt.Println("no active session")
				return nil
			}
			elapsedSec := int64(time.Since(active.StartedAt).Seconds())
			if elapsedSec < 0 {
				elapsedSec = 0
			}
			fmt.Printf(
				"active: @%s | %s | started %s | elapsed %s | note: %s\n",
				active.ClientName,
				active.TrackingTypeName,
				active.StartedAt.Local().Format("2006-01-02 15:04:05"),
				report.HumanDuration(elapsedSec),
				active.Note,
			)
			return nil
		}),
	}
	root.AddCommand(statusCmd)

	var reportClient, fromDate, toDate string
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Show report by client and date range",
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if reportClient == "" || fromDate == "" || toDate == "" {
				return errors.New("--client, --from, and --to are required")
			}
			reportClient = strings.TrimPrefix(reportClient, "@")
			from, to, err := report.ParseDateRange(fromDate, toDate)
			if err != nil {
				return err
			}
			rows, total, err := svc.Report(reportClient, from, to)
			if err != nil {
				return err
			}
			fmt.Printf("report for @%s (%s -> %s)\n", reportClient, fromDate, toDate)
			for _, r := range rows {
				fmt.Printf("- %s | %s | %s | %s\n", r.StartedAt.Local().Format("2006-01-02 15:04"), r.TrackingTypeName, report.HumanDuration(r.ComputedDurationS), r.Note)
			}
			fmt.Printf("total: %s\n", report.HumanDuration(total))
			return nil
		}),
	}
	reportCmd.Flags().StringVar(&reportClient, "client", "", "client name, with or without @")
	reportCmd.Flags().StringVar(&fromDate, "from", "", "from date (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&toDate, "to", "", "to date (YYYY-MM-DD)")
	root.AddCommand(reportCmd)

	var csvOut string
	exportCmd := &cobra.Command{Use: "export", Short: "Export data"}
	exportCSV := &cobra.Command{
		Use:   "csv",
		Short: "Export report to CSV",
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if reportClient == "" || fromDate == "" || toDate == "" || csvOut == "" {
				return errors.New("--client, --from, --to, and --out are required")
			}
			reportClient = strings.TrimPrefix(reportClient, "@")
			from, to, err := report.ParseDateRange(fromDate, toDate)
			if err != nil {
				return err
			}
			rows, _, err := svc.Report(reportClient, from, to)
			if err != nil {
				return err
			}
			if err := export.WriteReportCSV(csvOut, rows); err != nil {
				return err
			}
			fmt.Printf("exported report to %s\n", csvOut)
			return nil
		}),
	}
	exportCSV.Flags().StringVar(&reportClient, "client", "", "client name, with or without @")
	exportCSV.Flags().StringVar(&fromDate, "from", "", "from date (YYYY-MM-DD)")
	exportCSV.Flags().StringVar(&toDate, "to", "", "to date (YYYY-MM-DD)")
	exportCSV.Flags().StringVar(&csvOut, "out", "", "output csv file path")
	exportCmd.AddCommand(exportCSV)
	root.AddCommand(exportCmd)

	return root
}
