package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/maplepreneur/chrono/internal/export"
	"github.com/maplepreneur/chrono/internal/report"
	"github.com/maplepreneur/chrono/internal/service"
	sqlstore "github.com/maplepreneur/chrono/internal/store/sqlite"
	"github.com/maplepreneur/chrono/internal/tui"
	"github.com/maplepreneur/chrono/internal/update"
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
	root := &cobra.Command{
		Use:     "timmies",
		Short:   "Timmies TUI is a CLI/TUI time tracker",
		Long:    cliLogo() + "\nTimmies TUI is a CLI/TUI time tracker",
		Version: "1.0",
	}
	root.PersistentFlags().StringVar(&dbPath, "db", "tim.db", "sqlite database path")

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
	root.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Update timmies from the main branch on GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			remoteOutput, err := exec.Command("git", "config", "--get", "remote.origin.url").CombinedOutput()
			remoteURL := strings.TrimSpace(string(remoteOutput))
			if err != nil || remoteURL == "" {
				return errors.New("could not read git remote.origin.url; run this command from a cloned GitHub repository and ensure origin is configured")
			}
			target, err := update.InstallTargetFromRemote(remoteURL)
			if err != nil {
				return err
			}
			var installOutput bytes.Buffer
			installCmd := exec.Command("go", "install", target)
			installCmd.Stdout = &installOutput
			installCmd.Stderr = &installOutput
			if err := installCmd.Run(); err != nil {
				return fmt.Errorf("go install failed for %s:\n%s", target, strings.TrimSpace(installOutput.String()))
			}
			fmt.Printf("updated timmies from main branch\ninstalled target: %s\n", target)
			return nil
		},
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

	configCmd := &cobra.Command{Use: "config", Short: "Manage app configuration"}
	configSetNameCmd := &cobra.Command{
		Use:   "set-name [display-name]",
		Args:  cobra.ExactArgs(1),
		Short: "Set report branding display name",
		RunE: withDeps(func(store *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			displayName := strings.TrimSpace(args[0])
			if displayName == "" {
				return errors.New("display name cannot be empty")
			}
			if err := svc.SetBrandingDisplayName(displayName); err != nil {
				return err
			}
			fmt.Printf("set branding display name: %s\n", displayName)
			return nil
		}),
	}
	configSetLogoCmd := &cobra.Command{
		Use:   "set-logo [logo-path]",
		Args:  cobra.ExactArgs(1),
		Short: "Set report branding logo file path",
		RunE: withDeps(func(store *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			logoPath := strings.TrimSpace(args[0])
			if logoPath == "" {
				return errors.New("logo path cannot be empty")
			}
			if err := validateLogoPath(logoPath); err != nil {
				return err
			}
			if err := svc.SetBrandingLogoPath(logoPath); err != nil {
				return err
			}
			fmt.Printf("set branding logo path: %s\n", logoPath)
			return nil
		}),
	}
	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current branding configuration",
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			branding, err := svc.BrandingSettings()
			if err != nil {
				return err
			}
			displayName := branding.DisplayName
			if displayName == "" {
				displayName = "(not set)"
			}
			logoPath := branding.LogoPath
			if logoPath == "" {
				logoPath = "(not set)"
			}
			fmt.Printf("display name: %s\n", displayName)
			fmt.Printf("logo path: %s\n", logoPath)
			return nil
		}),
	}
	configCmd.AddCommand(configSetNameCmd, configSetLogoCmd, configShowCmd)
	root.AddCommand(configCmd)

	typeCmd := &cobra.Command{Use: "type", Short: "Manage tracking types"}
	var typeBillable bool
	var typeHourlyRate float64
	typeAddCmd := &cobra.Command{
		Use:   "add [name]",
		Args:  cobra.ExactArgs(1),
		Short: "Add a tracking type",
		Long:  "Add a tracking type with optional billing configuration.",
		Example: strings.Join([]string{
			"  timmies type add dev",
			"  timmies type add consulting --billable --rate 150",
		}, "\n"),
		RunE: withDeps(func(store *sqlstore.Store, _ *service.TimerService, cmd *cobra.Command, args []string) error {
			if typeHourlyRate < 0 {
				return errors.New("--rate must be zero or greater")
			}
			if !typeBillable && cmd.Flags().Changed("rate") && typeHourlyRate > 0 {
				return errors.New("--rate requires --billable")
			}
			if typeBillable && !cmd.Flags().Changed("rate") {
				return errors.New("--rate is required when --billable is set")
			}
			if typeBillable && typeHourlyRate == 0 {
				return errors.New("--rate must be greater than 0 when --billable is set")
			}
			if err := store.AddTrackingTypeWithBilling(args[0], typeBillable, typeHourlyRate); err != nil {
				return err
			}
			if typeBillable {
				fmt.Printf("added type: %s (billable @ $%.2f/h)\n", args[0], typeHourlyRate)
			} else {
				fmt.Printf("added type: %s (non-billable)\n", args[0])
			}
			return nil
		}),
	}
	typeAddCmd.Flags().BoolVar(&typeBillable, "billable", false, "mark tracking type as billable")
	typeAddCmd.Flags().Float64Var(&typeHourlyRate, "rate", 0, "hourly rate in dollars (required with --billable)")
	typeListCmd := &cobra.Command{
		Use:   "list",
		Short: "List tracking types",
		RunE: withDeps(func(store *sqlstore.Store, _ *service.TimerService, cmd *cobra.Command, args []string) error {
			types, err := store.ListTrackingTypeDetails()
			if err != nil {
				return err
			}
			for _, t := range types {
				if t.IsBillable {
					fmt.Printf("- %s | billable:$%.2f/h\n", t.Name, t.HourlyRate)
				} else {
					fmt.Printf("- %s | non-billable\n", t.Name)
				}
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

	var resourceSessionID int64
	var resourceName string
	var resourceCost float64
	sessionCmd := &cobra.Command{Use: "session", Short: "Manage sessions"}
	sessionResourceCmd := &cobra.Command{Use: "resource", Short: "Manage session resources"}
	sessionResourceAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Attach a resource cost to a session",
		Long:  "Attach a named resource cost entry to an existing session.",
		Example: strings.Join([]string{
			"  timmies session resource add --session 42 --name ai_tokens --cost 12.50",
			"  timmies session resource add --session 42 --name gpu_minutes --cost 7.25",
		}, "\n"),
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			resourceName = strings.TrimSpace(resourceName)
			if resourceSessionID <= 0 {
				return errors.New("--session must be greater than 0")
			}
			if resourceName == "" {
				return errors.New("--name is required")
			}
			if resourceCost < 0 {
				return errors.New("--cost must be zero or greater")
			}
			if err := svc.AddSessionResource(resourceSessionID, resourceName, resourceCost); err != nil {
				return err
			}
			fmt.Printf("added resource to session %d: %s ($%.2f)\n", resourceSessionID, resourceName, resourceCost)
			return nil
		}),
	}
	sessionResourceAddCmd.Flags().Int64Var(&resourceSessionID, "session", 0, "session id")
	sessionResourceAddCmd.Flags().StringVar(&resourceName, "name", "", "resource name")
	sessionResourceAddCmd.Flags().Float64Var(&resourceCost, "cost", 0, "resource cost in dollars")
	_ = sessionResourceAddCmd.MarkFlagRequired("session")
	_ = sessionResourceAddCmd.MarkFlagRequired("name")
	_ = sessionResourceAddCmd.MarkFlagRequired("cost")
	sessionResourceCmd.AddCommand(sessionResourceAddCmd)
	sessionCmd.AddCommand(sessionResourceCmd)
	root.AddCommand(sessionCmd)

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
			resources, err := svc.ListSessionResources(active.ID)
			if err != nil {
				return err
			}
			var resourceTotal float64
			for _, r := range resources {
				resourceTotal += r.CostAmount
			}
			fmt.Printf(
				"active: @%s | %s | started %s | elapsed %s | resources:$%.2f | note: %s\n",
				active.ClientName,
				active.TrackingTypeName,
				active.StartedAt.Local().Format("2006-01-02 15:04:05"),
				report.HumanDuration(elapsedSec),
				resourceTotal,
				active.Note,
			)
			if len(resources) > 0 {
				fmt.Println("resources:")
				for _, r := range resources {
					fmt.Printf("  - %s: $%.2f\n", r.ResourceName, r.CostAmount)
				}
			}
			return nil
		}),
	}
	root.AddCommand(statusCmd)

	var reportClient, fromDate, toDate string
	var lastDays, lastWeeks int
	var thisYear bool
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Show report by client and period",
		Long: strings.Join([]string{
			"Show a client report for either an explicit date range or one relative period.",
			"Use --from and --to together, or choose one of --last-days, --last-weeks, or --this-year.",
		}, "\n\n"),
		Example: strings.Join([]string{
			"  timmies report --client @acme --from 2026-01-01 --to 2026-01-31",
			"  timmies report --client @acme --last-days 7",
			"  timmies report --client @acme --this-year",
		}, "\n"),
		RunE: withDeps(func(_ *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if reportClient == "" {
				return errors.New("--client is required")
			}
			reportClient = strings.TrimPrefix(reportClient, "@")
			from, to, err := report.ResolveDateRange(report.PeriodOptions{
				FromDate:  fromDate,
				ToDate:    toDate,
				LastDays:  lastDays,
				LastWeeks: lastWeeks,
				ThisYear:  thisYear,
			}, time.Now())
			if err != nil {
				return err
			}
			rows, summary, err := svc.Report(reportClient, from, to)
			if err != nil {
				return err
			}
			fmt.Printf("report for @%s (%s -> %s)\n", reportClient, from.Format("2006-01-02"), to.Format("2006-01-02"))
			for _, r := range rows {
				fmt.Printf(
					"- %s | %s | %s | billable:%t rate:$%.2f/h time:$%.2f resources:$%.2f total:$%.2f | %s\n",
					r.StartedAt.Local().Format("2006-01-02 15:04"),
					r.TrackingTypeName,
					report.HumanDuration(r.ComputedDurationS),
					r.IsBillable,
					r.HourlyRate,
					r.BillableAmount,
					r.ResourceCostTotal,
					r.MonetaryTotal,
					r.Note,
				)
			}
			fmt.Printf(
				"total: %s | time:$%.2f resources:$%.2f combined:$%.2f\n",
				report.HumanDuration(summary.DurationSec),
				summary.TimeBillableTotal,
				summary.ResourceCostTotal,
				summary.MonetaryTotal,
			)
			return nil
		}),
	}
	reportCmd.Flags().StringVar(&reportClient, "client", "", "client name, with or without @")
	reportCmd.Flags().StringVar(&fromDate, "from", "", "from date (YYYY-MM-DD)")
	reportCmd.Flags().StringVar(&toDate, "to", "", "to date (YYYY-MM-DD)")
	reportCmd.Flags().IntVar(&lastDays, "last-days", 0, "relative range: last N days")
	reportCmd.Flags().IntVar(&lastWeeks, "last-weeks", 0, "relative range: last N weeks")
	reportCmd.Flags().BoolVar(&thisYear, "this-year", false, "relative range: from Jan 1 of current year through today")
	root.AddCommand(reportCmd)

	var exportOut string
	exportCmd := &cobra.Command{Use: "export", Short: "Export data"}
	exportCSV := &cobra.Command{
		Use:   "csv",
		Short: "Export report to CSV",
		RunE: withDeps(func(store *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if reportClient == "" || exportOut == "" {
				return errors.New("--client and --out are required")
			}
			reportClient = strings.TrimPrefix(reportClient, "@")
			from, to, err := report.ResolveDateRange(report.PeriodOptions{
				FromDate:  fromDate,
				ToDate:    toDate,
				LastDays:  lastDays,
				LastWeeks: lastWeeks,
				ThisYear:  thisYear,
			}, time.Now())
			if err != nil {
				return err
			}
			rows, _, err := svc.Report(reportClient, from, to)
			if err != nil {
				return err
			}
			if err := export.WriteReportCSV(exportOut, rows); err != nil {
				return err
			}
			fmt.Printf("exported report to %s\n", exportOut)
			return nil
		}),
	}
	exportPDF := &cobra.Command{
		Use:   "pdf",
		Short: "Export report to PDF",
		RunE: withDeps(func(store *sqlstore.Store, svc *service.TimerService, cmd *cobra.Command, args []string) error {
			if reportClient == "" || exportOut == "" {
				return errors.New("--client and --out are required")
			}
			reportClient = strings.TrimPrefix(reportClient, "@")
			from, to, err := report.ResolveDateRange(report.PeriodOptions{
				FromDate:  fromDate,
				ToDate:    toDate,
				LastDays:  lastDays,
				LastWeeks: lastWeeks,
				ThisYear:  thisYear,
			}, time.Now())
			if err != nil {
				return err
			}
			rows, summary, err := svc.Report(reportClient, from, to)
			if err != nil {
				return err
			}
			branding, err := store.GetBrandingSettings()
			if err != nil {
				return err
			}
			if err := export.WriteReportPDF(exportOut, reportClient, from, to, rows, summary, export.ReportBranding{
				DisplayName: branding.DisplayName,
				LogoPath:    branding.LogoPath,
			}); err != nil {
				return err
			}
			fmt.Printf("exported report to %s\n", exportOut)
			return nil
		}),
	}
	exportCSV.Flags().StringVar(&reportClient, "client", "", "client name, with or without @")
	exportCSV.Flags().StringVar(&fromDate, "from", "", "from date (YYYY-MM-DD)")
	exportCSV.Flags().StringVar(&toDate, "to", "", "to date (YYYY-MM-DD)")
	exportCSV.Flags().IntVar(&lastDays, "last-days", 0, "relative range: last N days")
	exportCSV.Flags().IntVar(&lastWeeks, "last-weeks", 0, "relative range: last N weeks")
	exportCSV.Flags().BoolVar(&thisYear, "this-year", false, "relative range: from Jan 1 of current year through today")
	exportCSV.Flags().StringVar(&exportOut, "out", "", "output file path")

	exportPDF.Flags().StringVar(&reportClient, "client", "", "client name, with or without @")
	exportPDF.Flags().StringVar(&fromDate, "from", "", "from date (YYYY-MM-DD)")
	exportPDF.Flags().StringVar(&toDate, "to", "", "to date (YYYY-MM-DD)")
	exportPDF.Flags().IntVar(&lastDays, "last-days", 0, "relative range: last N days")
	exportPDF.Flags().IntVar(&lastWeeks, "last-weeks", 0, "relative range: last N weeks")
	exportPDF.Flags().BoolVar(&thisYear, "this-year", false, "relative range: from Jan 1 of current year through today")
	exportPDF.Flags().StringVar(&exportOut, "out", "", "output file path")
	exportCmd.AddCommand(exportCSV, exportPDF)
	root.AddCommand(exportCmd)

	return root
}

func validateLogoPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("logo file does not exist: %s", path)
		}
		return fmt.Errorf("cannot access logo file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("logo path must be a file, got directory: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read logo file %s: %w", path, err)
	}
	_ = f.Close()
	return nil
}

func cliLogo() string {
	red := "\x1b[31m"
	white := "\x1b[1;97m"
	dim := "\x1b[2m"
	reset := "\x1b[0m"
	return strings.Join([]string{
		red + "             /\\" + reset,
		red + "            _/  \\_" + reset,
		red + "           /      \\" + reset,
		red + "   _      /        \\      _" + reset,
		red + "  / \\    /          \\    / \\" + reset,
		red + " /   \\__/            \\__/   \\" + reset,
		red + " \\          " + white + "TIMMIES" + reset + red + "         /" + reset,
		red + "  \\                        /" + reset,
		red + "   \\                      /" + reset,
		red + "    \\                    /" + reset,
		red + "     \\__              __/" + reset,
		red + "        \\____    ____/" + reset,
		red + "             |  |" + reset,
		red + "             |  |" + reset,
		"",
		dim + "  Created with ❤️  by Voxel North Technologies Inc." + reset,
		dim + "  Licensed under the O'Saasy License" + reset,
	}, "\n")
}
