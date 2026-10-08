package main

import (
	"github.com/alderon07/al/internal/app"

	"github.com/alderon07/al/internal/entry"

	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"runtime/debug"

	"strings"
	"time"
)

var version = "dev"

//go:embed web/*
var web embed.FS

type Alias = entry.Alias

func main() {
	os.Exit(runMain())
}

func runMain() int {
	if len(os.Args) == 1 {
		runTUI()
		return 0
	}
	if os.Args[1] == "help" || isHelpFlag(os.Args[1]) {
		if len(os.Args) == 2 {
			printUsage()
		} else if len(os.Args) == 3 {
			printCommandUsage(os.Args[2])
		} else {
			fmt.Fprintln(os.Stderr, "Usage: al help [COMMAND]")
			return 2
		}
		return 0
	}
	if len(os.Args) == 3 && isHelpFlag(os.Args[2]) {
		printCommandUsage(os.Args[1])
		return 0
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("alias-lens %s\n", displayVersion())
	case "--web":
		if len(os.Args) > 2 {
			printUsage()
			return 2
		}
		if err := runWeb(); err != nil {
			cliError(err)
			return 1
		}
	case "repo":
		if len(os.Args) == 2 {
			if err := runRepoPicker(""); err != nil {
				cliError(err)
				return 1
			}
			return 0
		}
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: al repo [/path/to/dotfiles]")
			return 2
		}
		if os.Args[2] == "github" || os.Args[2] == "bitbucket" || os.Args[2] == "gitlab" {
			if err := runRepoPicker(os.Args[2]); err != nil {
				cliError(err)
				return 1
			}
			return 0
		}
		if err := applicationServices().ConfigureRepository(os.Args[2]); err != nil {
			cliError(err)
			return 1
		}
		config, err := loadConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Alias Lens could not read its settings:", err)
			return 1
		}
		fmt.Printf("Alias Lens will sync only %s in %s\n", config.AliasFile, os.Args[2])
	case "config":
		if err := runConfigCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "data":
		if err := runDataCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "context":
		if err := runContextCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "init":
		if err := runInitCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
		return 0
	case "catalog-loader":
		if err := runCatalogLoader(os.Args[2:]); err != nil {
			return 1
		}
		return 0
	case "catalog":
		exitCode, err := runCatalogCommand(os.Args[2:])
		if err != nil {
			cliError(err)
			if exitCode == 0 {
				return 1
			}
		}
		return exitCode
	case "theme":
		if err := runThemeCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "shortcuts":
		if err := runShortcutsCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "status":
		exitCode, err := runStatusCommand(os.Args[2:])
		if err != nil {
			cliError(err)
		}
		return exitCode
	case "plan":
		exitCode, err := runPlanCommand(os.Args[2:])
		if err != nil {
			cliError(err)
		}
		return exitCode
	case "completion":
		if err := runCompletionCommand(os.Args[2:], os.Stdout); err != nil {
			cliError(err)
			if strings.HasPrefix(err.Error(), "usage:") || strings.HasPrefix(err.Error(), "unsupported shell") {
				return 2
			}
			return 1
		}
	case "completion-candidates":
		if !runCompletionCandidates(os.Args[2:], os.Stdout) {
			return 1
		}
	case "pick":
		commandOnly := false
		executeSelection := false
		arguments := os.Args[2:]
		for len(arguments) > 0 {
			switch arguments[0] {
			case "--command":
				commandOnly = true
				arguments = arguments[1:]
			case "--execute":
				executeSelection = true
				arguments = arguments[1:]
			default:
				goto pickerArgumentsParsed
			}
		}
	pickerArgumentsParsed:
		if len(arguments) > 1 {
			fmt.Fprintln(os.Stderr, "Usage: al pick [--command] [--execute] [QUERY]")
			return 2
		}
		query := ""
		if len(arguments) == 1 {
			query = arguments[0]
		}
		if err := runAliasPicker(query, commandOnly, executeSelection); err != nil {
			cliError(err)
			return 1
		}
	case "shell-init":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: al shell-init bash|zsh")
			return 2
		}
		if err := printShellIntegration(os.Args[2]); err != nil {
			cliError(err)
			return 1
		}
	case "shell-entry":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: alias-lens shell-entry NAME")
			return 2
		}
		if err := printShellEntry(os.Args[2]); err != nil {
			cliError(err)
			return 1
		}
	case "suggest":
		if err := runHistorySuggestions(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "search":
		if err := runSearchCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "stats":
		if err := runStatsCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "record-use":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "Usage: alias-lens record-use NAME")
			return 2
		}
		if err := applicationServices().RecordAliasUse(os.Args[2]); err != nil {
			cliError(err)
			return 1
		}
	case "export":
		if err := runExportCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "import":
		if err := runImportCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "scan":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al scan")
			return 2
		}
		if err := runSecretScan(); err != nil {
			cliError(err)
			return 1
		}
	case "check":
		exitCode, err := runAliasCheck(os.Args[2:])
		if err != nil {
			cliError(err)
			if exitCode == 0 {
				return 1
			}
		}
		return exitCode
	case "meta":
		if err := runMetadataCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "describe":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al describe")
			return 2
		}
		if err := addAliasDescriptions(); err != nil {
			cliError(err)
			return 1
		}
	case "history":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al history")
			return 2
		}
		if err := runRevisionHistory(); err != nil {
			cliError(err)
			return 1
		}
	case "undo":
		if len(os.Args) > 3 {
			fmt.Fprintln(os.Stderr, "Usage: al undo [REVISION]")
			return 2
		}
		revision := "latest"
		if len(os.Args) == 3 {
			revision = os.Args[2]
		}
		if err := restoreRevision(revision); err != nil {
			cliError(err)
			return 1
		}
	case "doctor":
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al doctor")
			return 2
		}
		if err := runDoctor(); err != nil {
			cliError(err)
			return 1
		}
	case "setup":
		if err := runSetupCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "autosync":
		if err := runAutoSyncCommand(os.Args[2:]); err != nil {
			cliError(err)
			return 1
		}
	case "watch":
		if len(os.Args) == 3 && os.Args[2] == "--ensure" {
			config, err := loadConfig()
			if err == nil && config.AutoSync.Enabled {
				err = applicationServices().EnsureWatchProcess()
			}
			if err != nil {
				cliError(err)
				return 1
			}
			return 0
		}
		daemon := len(os.Args) == 3 && os.Args[2] == "--daemon"
		if len(os.Args) > 3 || (len(os.Args) == 3 && !daemon) {
			fmt.Fprintln(os.Stderr, "Usage: al watch")
			return 2
		}
		if err := applicationServices().RunWatch(daemon); err != nil {
			if !daemon {
				cliError(err)
			}
			return 1
		}
	case "track", "untrack":
		if err := runTrackCommand(os.Args[2:], os.Args[1] == "untrack"); err != nil {
			cliError(err)
			return 1
		}
	case "sync":
		if handled, err := runCatalogSyncCommand(os.Args[2:]); handled {
			if err != nil {
				cliError(err)
				return 1
			}
			return 0
		}
		if len(os.Args) > 3 || (len(os.Args) == 3 && os.Args[2] != "--push" && os.Args[2] != "--pull") {
			printUsage()
			return 2
		}
		if len(os.Args) == 3 && os.Args[2] == "--pull" {
			message, err := withCLIProgress("Pulling aliases", applicationServices().PullRepository)
			if err != nil {
				cliError(err)
				return 1
			}
			cliResult(message)
			return 0
		}
		push := len(os.Args) == 3 && os.Args[2] == "--push"
		message, err := withCLIProgress("Syncing aliases", func() (string, error) { return applicationServices().SyncRepository(push) })
		if err != nil {
			cliError(err)
			return 1
		}
		cliResult(message)
	case "diff":
		if len(os.Args) == 3 && os.Args[2] == "--tui" {
			if err := runTUIWithDiff(true); err != nil {
				cliError(err)
				return 1
			}
			return 0
		}
		if len(os.Args) != 2 {
			fmt.Fprintln(os.Stderr, "Usage: al diff [--tui]")
			return 2
		}
		if err := showRepositoryDiff(); err != nil {
			cliError(err)
			return 1
		}
	default:
		printUsage()
		return 2
	}
	return 0
}

func displayVersion() string {
	moduleVersion := ""
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = buildInfo.Main.Version
	}
	return resolveVersion(version, moduleVersion)
}

func resolveVersion(injected, moduleVersion string) string {
	if injected != "" && injected != "dev" {
		return strings.TrimPrefix(injected, "v")
	}
	if moduleVersion != "" && moduleVersion != "(devel)" {
		return strings.TrimPrefix(moduleVersion, "v")
	}
	return "dev"
}

func printShellIntegration(name string) error {
	adapter, err := applicationServices().ShellAdapter(name)
	if err != nil {
		return err
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	integration, err := shellIntegrationForConfig(adapter, config)
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(integration)
	return err
}

func shellIntegrationForConfig(adapter app.ShellAdapter, config AppConfig) (string, error) {
	letter, err := parseLauncherKey(launcherLabel(config))
	if err != nil {
		return "", err
	}
	integration := adapter.Integration()
	if adapter.Name() == "bash" {
		integration = strings.ReplaceAll(integration, `\C-g`, `\C-`+strings.ToLower(letter))
		binding := `bind '"\C-` + strings.ToLower(letter) + `":"\C-x\C-` + strings.ToLower(letter) + `\C-x\C-a"'`
		if !strings.Contains(integration, binding) {
			return "", fmt.Errorf("Bash integration is missing its launcher binding")
		}
		integration = strings.Replace(integration, binding, binding+"\n  _alias_lens_bound_key='\\C-"+strings.ToLower(letter)+"'", 1)
		integration = "if [ -n \"${_alias_lens_bound_key-}\" ]; then\n  bind -r \"$_alias_lens_bound_key\"\n  unset _alias_lens_bound_key\nfi\n" + integration
	} else {
		integration = strings.ReplaceAll(integration, "^G", "^"+letter)
		binding := "bindkey '^" + letter + "' _alias_lens_launch"
		if !strings.Contains(integration, binding) {
			return "", fmt.Errorf("Zsh integration is missing its launcher binding")
		}
		integration = strings.Replace(integration, binding, binding+"\n  _alias_lens_bound_key='^"+letter+"'", 1)
		integration = "if [[ -n \"${_alias_lens_bound_key-}\" ]]; then\n  bindkey -r \"$_alias_lens_bound_key\"\n  unset _alias_lens_bound_key\nfi\n" + integration
	}
	integration = strings.ReplaceAll(integration, "Ctrl+G", launcherLabel(config))
	return integration, nil
}

func runWeb() error {
	if len(os.Args) > 2 {
		printUsage()
		return nil
	}

	address := "127.0.0.1:8787"
	token, err := generateWebToken()
	if err != nil {
		return fmt.Errorf("create web session: %w", err)
	}
	handler, err := newWebHandler(token, address)
	if err != nil {
		return err
	}

	fmt.Printf("Alias Lens web mode is running at http://%s/#token=%s\n", address, token)
	fmt.Println("Reading aliases from", applicationServices().AliasDisplayPath())
	return serveLocalWeb(address, handler)
}

func serveLocalWeb(address string, handler http.Handler) error {
	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("start web server: %w; check whether port 8787 is already in use", err)
	}
	return nil
}

func generateWebToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func newWebHandler(token, expectedHost string) (http.Handler, error) {
	return newWebHandlerWithCatalogDiff(token, expectedHost, nil)
}

func newWebHandlerWithCatalogDiff(token, expectedHost string, catalogDiff *catalogDiffWebView) (http.Handler, error) {
	static, err := fs.Sub(web, "web")
	if err != nil {
		return nil, fmt.Errorf("load web assets: %w", err)
	}
	mux := http.NewServeMux()
	requireAuthentication := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("Authorization")
			expected := "Bearer " + token
			if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "web session authentication required", http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	if catalogDiff == nil {
		mux.HandleFunc("/api/aliases", requireAuthentication(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			aliasesHandler(w, r)
		}))
	} else {
		mux.HandleFunc("/api/catalog-diff", requireAuthentication(catalogDiff.summaryHandler))
		mux.HandleFunc("/api/catalog-diff/details", requireAuthentication(catalogDiff.detailHandler))
	}
	staticHandler := http.FileServer(http.FS(static))
	if catalogDiff == nil {
		mux.Handle("/", staticHandler)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				copy := r.Clone(r.Context())
				copy.URL.Path = "/diff.html"
				staticHandler.ServeHTTP(w, copy)
			case "/diff.html", "/diff.css", "/diff.js":
				staticHandler.ServeHTTP(w, r)
			default:
				http.NotFound(w, r)
			}
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; font-src 'self'; worker-src 'self'; img-src 'none'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Host != expectedHost {
			http.Error(w, "invalid Host header", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+expectedHost {
			http.Error(w, "invalid Origin header", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	}), nil
}

func aliasesHandler(w http.ResponseWriter, r *http.Request) {
	aliases, err := applicationServices().Entries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(aliases)
}
