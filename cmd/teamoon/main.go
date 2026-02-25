package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/dashboard"
	"github.com/JuanVilla424/teamoon/internal/engine"
	"github.com/JuanVilla424/teamoon/internal/logs"
	"github.com/JuanVilla424/teamoon/internal/onboarding"
	"github.com/JuanVilla424/teamoon/internal/pathutil"
	"github.com/JuanVilla424/teamoon/internal/queue"
	"github.com/JuanVilla424/teamoon/internal/web"
)

var (
	version  = "1.1.0"
	buildNum = "0"
)

func main() {
	pathutil.AugmentPath()

	dashboard.Version = version
	dashboard.BuildNum = buildNum
	web.Version = version
	web.BuildNum = buildNum

	var debugFlag bool

	rootCmd := &cobra.Command{
		Use:     "teamoon",
		Short:   "AI-powered task autopilot",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			if debugFlag {
				cfg.Debug = true
			}

			engineMgr := engine.NewManager()
			logBuf := logs.NewRingBuffer(100)
			logBuf.SetDebug(cfg.Debug)
			if f := logBuf.File(); f != nil {
				log.SetOutput(f)
			}

			m := dashboard.NewModel(cfg, engineMgr, logBuf)
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage pending tasks",
	}

	var taskProject string
	var taskPriority string

	taskAddCmd := &cobra.Command{
		Use:   "add [description]",
		Short: "Add a new task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			desc := args[0]
			t, err := queue.Add(taskProject, desc, taskPriority)
			if err != nil {
				return err
			}
			fmt.Printf("Task #%d added: [%s] %s — %s\n", t.ID, t.Priority, t.Project, t.Description)
			return nil
		},
	}
	taskAddCmd.Flags().StringVarP(&taskProject, "project", "p", "", "Project name")
	taskAddCmd.Flags().StringVarP(&taskPriority, "priority", "r", "med", "Priority: high, med, low")

	taskDoneCmd := &cobra.Command{
		Use:   "done [id]",
		Short: "Mark a task as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var id int
			if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
				return fmt.Errorf("invalid task ID: %s", args[0])
			}
			if err := queue.MarkDone(id); err != nil {
				return err
			}
			fmt.Printf("Task #%d marked as done\n", id)
			return nil
		},
	}

	taskListCmd := &cobra.Command{
		Use:   "list",
		Short: "List pending tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := queue.ListPending()
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Println("No pending tasks")
				return nil
			}
			for _, t := range tasks {
				fmt.Printf("#%-3d [%-4s] %-20s %s\n", t.ID, t.Priority, t.Project, t.Description)
			}
			return nil
		},
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Run web server only (no TUI)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			cfg.WebEnabled = true
			if debugFlag {
				cfg.Debug = true
			}

			engineMgr := engine.NewManager()
			logBuf := logs.NewRingBuffer(100)
			logBuf.SetDebug(cfg.Debug)
			if f := logBuf.File(); f != nil {
				log.SetOutput(f)
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			config.InitMCPFromGlobal(&cfg)
			srv := web.NewServer(cfg, engineMgr, logBuf)
			go srv.Start(ctx)

			log.Printf("[serve] v%s #%s on :%d", version, buildNum, cfg.WebPort)
			<-ctx.Done()
			log.Println("[serve] shutting down")
			return nil
		},
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive onboarding wizard for Claude Code global setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return onboarding.Run()
		},
	}

	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enable debug logging")

	taskCmd.AddCommand(taskAddCmd, taskDoneCmd, taskListCmd)
	rootCmd.AddCommand(taskCmd, serveCmd, initCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
