package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"bufio"
	"github.com/404kernel/bound/pkg/config"
	"github.com/404kernel/bound/pkg/governor"
	"github.com/404kernel/bound/pkg/ledger"
	"github.com/404kernel/bound/pkg/notify"
	"github.com/404kernel/bound/pkg/proxy"
	"github.com/404kernel/bound/pkg/vault"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const Banner = `
  ██████╗  ██████╗ ██╗   ██╗███╗   ██╗██████╗ 
  ██╔══██╗██╔═══██╗██║   ██║████╗  ██║██╔══██╗
  ██████╔╝██║   ██║██║   ██║██╔██╗ ██║██║  ██║
  ██╔══██╗██║   ██║██║   ██║██║╚██╗██║██║  ██║
  ██████╔╝╚██████╔╝╚██████╔╝██║ ╚████║██████╔╝
  ╚═════╝  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝╚═════╝ 
  The Programmable Spend Vault & Financial Rails for Autonomous AI Agents
`

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	command := os.Args[1]

	switch command {
	case "proxy":
		runProxy(os.Args[2:])
	case "run":
		runWrappedAgent(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "status":
		runStatus()
	case "audit":
		runAudit()
	case "reset":
		runReset()
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Printf("Unknown command '%s'. Run 'bound help' for usage.\n", command)
	}
}

func printHelp() {
	fmt.Print(Banner)
	fmt.Print(`
Usage:
  bound <command> [options]

Commands:
  proxy       Start the Bound Governance Proxy daemon on localhost:8443
  run         Execute an AI agent command wrapped in strict budget guardrails
  status      Display live vault spend, remaining budget, and circuit breaker status
  audit       Inspect transaction history and decision logs
  reset       Reset vault spend and clear tripped circuit breakers
  help        Display this reference guide

Examples:
  # Start the proxy daemon with a $20 max budget and $5/hr velocity limit:
  bound proxy --port :8443 --budget 20.00 --hourly 5.00

  # Run any Python or Node agent wrapped with a $5.00 hard spend limit:
  bound run --budget 5.00 -- python agent.py
`)
}

func getPaths() (string, string) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".bound")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "vault.json"), filepath.Join(dir, "audit.jsonl")
}

func runProxy(args []string) {
	fs := flag.NewFlagSet("proxy", flag.ExitOnError)
	port := fs.String("port", ":8443", "Proxy listen address")
	budget := fs.Float64("budget", 25.00, "Max lifetime budget in USD")
	hourly := fs.Float64("hourly", 5.00, "Max hourly spend velocity limit in USD")
	daily := fs.Float64("daily", 10.00, "Max 24h spend limit in USD")
	upstream := fs.String("upstream", "https://api.openai.com", "Default upstream API endpoint")
	webhook := fs.String("webhook", "", "Optional webhook URL for escrow hold alerts (Slack / Discord)")
	tgBot := fs.String("telegram-bot", "", "Optional Telegram Bot Token for escrow push notifications")
	tgChat := fs.String("telegram-chat", "", "Optional Telegram Chat ID for escrow push notifications")
	_ = fs.Parse(args)

	fmt.Print(Banner)
	vaultPath, auditPath := getPaths()

	policy := vault.Policy{
		MaxTotal:            *budget,
		MaxHourly:           *hourly,
		MaxDaily:            *daily,
		MaxPerRequest:       10.00,
		RequireConfirmAbove: 15.00,
	}

	v := vault.NewVault(policy, vaultPath)
	gov := governor.NewGovernor(governor.DefaultConfig(), v)
	led := ledger.NewLedger(auditPath)

	cfg := config.LoadConfig()
	if *webhook == "" {
		*webhook = cfg.WebhookURL
	}
	if *tgBot == "" {
		*tgBot = cfg.TelegramBot
	}
	if *tgChat == "" {
		*tgChat = cfg.TelegramChatID
	}
	if *upstream == "https://api.openai.com" && cfg.UpstreamAPI != "" {
		*upstream = cfg.UpstreamAPI
	}

	var notif *notify.Notifier
	if *webhook != "" || (*tgBot != "" && *tgChat != "") {
		notif = notify.NewNotifier(*webhook, *tgBot, *tgChat)
	}

	srv, err := proxy.NewServerWithNotifier(*port, gov, v, led, notif, *upstream)
	if err != nil {
		fmt.Printf("Failed to initialize Bound proxy: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[BOUND // ACTIVE] Governance daemon listening on %s\n", *port)
	fmt.Printf("Dashboard       : http://localhost%s/dashboard\n", *port)
	fmt.Printf("Budget Caps     : Total: $%.2f | Daily: $%.2f | Hourly: $%.2f\n", policy.MaxTotal, policy.MaxDaily, policy.MaxHourly)
	fmt.Printf("Upstream Rail   : %s\n", *upstream)
	fmt.Println("Press Ctrl+C to shut down.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🔴 [BOUND] Shutting down governance proxy...")
		_ = srv.Close()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("Proxy error: %v\n", err)
	}
}

func runWrappedAgent(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	budget := fs.Float64("budget", 10.00, "Max spend limit for this execution in USD")
	hourly := fs.Float64("hourly", 5.00, "Max hourly spend velocity in USD")
	_ = fs.Parse(args)

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		fmt.Println("Error: specify a command to run after options. Example: bound run --budget 10.00 -- python agent.py")
		return
	}

	vaultPath, auditPath := getPaths()
	policy := vault.Policy{
		MaxTotal:            *budget,
		MaxHourly:           *hourly,
		MaxDaily:            *budget,
		MaxPerRequest:       *budget,
		RequireConfirmAbove: *budget,
	}

	v := vault.NewVault(policy, vaultPath)
	gov := governor.NewGovernor(governor.DefaultConfig(), v)
	led := ledger.NewLedger(auditPath)

	port := ":8443"
	srv, err := proxy.NewServer(port, gov, v, led, "https://api.openai.com")
	if err != nil {
		fmt.Printf("Failed to initialize proxy: %v\n", err)
		return
	}

	go func() {
		_ = srv.Start()
	}()
	defer srv.Close()

	// Wait 100ms for proxy startup
	time.Sleep(100 * time.Millisecond)

	fmt.Print(Banner)
	fmt.Printf("🛡️ [BOUND RUN] Enforcing $%.2f budget cap on command: %s\n\n", *budget, strings.Join(cmdArgs, " "))

	targetCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	targetCmd.Stdout = os.Stdout
	targetCmd.Stderr = os.Stderr
	targetCmd.Stdin = os.Stdin

	// Inject proxy environment variables
	targetCmd.Env = append(os.Environ(),
		"HTTP_PROXY=http://localhost:8443",
		"HTTPS_PROXY=http://localhost:8443",
		"OPENAI_BASE_URL=http://localhost:8443/v1",
		"ANTHROPIC_BASE_URL=http://localhost:8443",
	)

	start := time.Now()
	err = targetCmd.Run()
	duration := time.Since(start)

	fmt.Println("\n------------------------------------------------------------")
	fmt.Printf("🏁 [BOUND SESSION FINISHED] Run Duration: %s\n", duration.Round(time.Millisecond))
	fmt.Printf("💰 Total Spent: $%.4f | Remaining Budget: $%.4f\n", v.TotalSpent, v.Remaining())
	if err != nil {
		fmt.Printf("Exit Code/Error: %v\n", err)
	}
}

func runStatus() {
	vaultPath, _ := getPaths()
	v := vault.NewVault(vault.DefaultPolicy(), vaultPath)
	s := v.Summary()

	fmt.Print(Banner)
	fmt.Println("╭──────────────────────────────────────────────────────────╮")
	fmt.Println("│ 📊 BOUND GOVERNANCE TELEMETRY & VAULT STATUS            │")
	fmt.Println("├──────────────────────────────────────────────────────────┤")
	fmt.Printf("│  Total Spent       : $%-34.4f │\n", s["total_spent"])
	fmt.Printf("│  Remaining Budget  : $%-34.4f │\n", s["remaining_budget"])
	fmt.Printf("│  Max Lifetime Cap  : $%-34.2f │\n", s["max_total"])
	fmt.Printf("│  Daily Limit       : $%-34.2f │\n", s["max_daily"])
	fmt.Printf("│  Hourly Velocity   : $%-34.2f │\n", s["max_hourly"])
	fmt.Printf("│  Total Transactions: %-35d │\n", s["total_transactions"])
	fmt.Printf("│  Active Agent Pools: %-35d │\n", s["active_agents"])
	fmt.Println("╰──────────────────────────────────────────────────────────╯")
}

func runAudit() {
	_, auditPath := getPaths()
	data, err := os.ReadFile(auditPath)
	if err != nil || len(data) == 0 {
		fmt.Println("No audit records found. Vault is clean ✨")
		return
	}

	fmt.Print(Banner)
	fmt.Println("📜 [BOUND AUDIT LOGS] Most recent entries:")
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	start := 0
	if len(lines) > 10 {
		start = len(lines) - 10
	}

	for _, line := range lines[start:] {
		fmt.Println("  " + line)
	}
}

func runReset() {
	vaultPath, _ := getPaths()
	v := vault.NewVault(vault.DefaultPolicy(), vaultPath)
	if err := v.Reset(); err != nil {
		fmt.Printf("Failed to reset vault: %v\n", err)
		return
	}
	fmt.Println("✨ [BOUND] Vault balances, transaction records, and circuit breakers reset successfully.")
}

func runSetup(args []string) {
	fmt.Print(Banner)
	fmt.Println("🛠️  [BOUND SETUP WIZARD] Configure Alert Channels & Integration Keys")
	fmt.Println("--------------------------------------------------------------------")

	cfg := config.LoadConfig()
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n[1/3] Telegram Bot Setup\n")
	fmt.Printf("Current Bot Token: %s\n", maskSecret(cfg.TelegramBot))
	fmt.Print("Enter Telegram Bot Token (or press Enter to keep): ")
	botToken, _ := reader.ReadString('\n')
	botToken = strings.TrimSpace(botToken)
	if botToken != "" {
		cfg.TelegramBot = botToken
	}

	fmt.Printf("Current Chat ID: %s\n", cfg.TelegramChatID)
	fmt.Print("Enter Telegram Chat ID (or press Enter to keep): ")
	chatID, _ := reader.ReadString('\n')
	chatID = strings.TrimSpace(chatID)
	if chatID != "" {
		cfg.TelegramChatID = chatID
	}

	if cfg.TelegramBot != "" && cfg.TelegramChatID != "" {
		fmt.Print("Testing Telegram connection... ")
		n := notify.NewNotifier("", cfg.TelegramBot, cfg.TelegramChatID)
		if err := n.TestTelegram(); err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		} else {
			fmt.Println("✅ Success! Check Telegram for the test ping.")
		}
	}

	fmt.Printf("\n[2/3] Discord / Slack Webhook Setup\n")
	fmt.Printf("Current Webhook: %s\n", maskSecret(cfg.WebhookURL))
	fmt.Print("Enter Webhook URL (Discord/Slack, or press Enter to keep): ")
	webhookURL, _ := reader.ReadString('\n')
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL != "" {
		cfg.WebhookURL = webhookURL
	}

	if cfg.WebhookURL != "" {
		fmt.Print("Testing Webhook dispatch... ")
		n := notify.NewNotifier(cfg.WebhookURL, "", "")
		if err := n.TestWebhook(); err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		} else {
			fmt.Println("✅ Success! Check your Discord / Slack channel.")
		}
	}

	fmt.Printf("\n[3/3] Default Upstream AI Endpoint\n")
	fmt.Printf("Current Upstream: %s\n", cfg.UpstreamAPI)
	fmt.Print("Enter Upstream API (default: https://api.openai.com, or Enter to keep): ")
	upstream, _ := reader.ReadString('\n')
	upstream = strings.TrimSpace(upstream)
	if upstream != "" {
		cfg.UpstreamAPI = upstream
	}

	if err := config.SaveConfig(cfg); err != nil {
		fmt.Printf("❌ Failed to save config: %v\n", err)
		return
	}

	fmt.Printf("\n🎉 Configuration saved to %s\n", config.ConfigPath())
	fmt.Println("Run 'bound proxy' now to govern agent traffic with your configured channels.")
}

func maskSecret(s string) string {
	if s == "" {
		return "(not configured)"
	}
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
