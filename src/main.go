package main

import "fmt"
import "encoding/json"
import "path/filepath"
import "os"
import "net"
import "strings"
import "bufio"
import "errors"
import "database/sql"
import _ "github.com/mattn/go-sqlite3"
import "golang.org/x/term"
import "sync"
import "time"

const (
	SocketPath = "/tmp/boring.sock"
	Clear      = "\033[H\033[2J"
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Inverse    = "\033[7m"
	Cyan       = "\033[36m"
)

const (
    ViewDashboard = iota
    ViewLogs
)

type Build struct {
	ID        int    `json:"id"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Commit    string `json:"commit_hash"`
	Pipeline  string `json:"pipeline"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	StartedAt  *time.Time `json:"started_at"` 
	FinishedAt *time.Time `json:"finished_at"`
}

type DashboardState struct {
	Builds        []Build
	Logs          []string
	SelectedIndex int
	ActiveView		int
	LogScrollIdx	int
	Mu            sync.Mutex
}

var db *sql.DB

func main() {
	if len(os.Args) < 2 {
		fmt.Println("No args supplied. Check --help")

		os.Exit(1)
	}

	switch os.Args[1] {
		case "--help":
			printHelp()
		case "trigger":
			callDaemon("trigger " + os.Args[2] + " " + os.Args[3] + " " + os.Args[4] + " " + os.Args[5]);
		case "dashboard":
			runDashboardCLI();
		case "-d":
			daemonMode()
		default:
			fmt.Println("Unknown command");
	}
}

func printHelp() {
	fmt.Println("Usage: boring-ci <command> [<args>]")
	fmt.Println("Commands:")
	fmt.Println("  trigger        Trigger a new build")
	fmt.Println("    --repo       Name (not url) of repo as added in your ~/.confg/repos.json")
	fmt.Println("    --branch     Branch name")
	fmt.Println("    --commit     (HEAD) Commit hash to build")
	fmt.Println("    --pipeline   Name of pipeline as you have in your repo's .boring-ci/pipelines/*")
	fmt.Println("  cancel         Cancel build(s). You can use any or all of the following params to cancel a build or builds that apply")
	fmt.Println("    --repo       Name (not url) of repo as added in your ~/.confg/repos.json")
	fmt.Println("    --branch     Branch name")
	fmt.Println("    --commit     (HEAD) Commit hash to build")
	fmt.Println("    --pipeline   Name of pipeline as you have in your repo's .boring-ci/pipelines/*")
	fmt.Println("    --id         Cancel by build ID")
	fmt.Println("  dashboard      Open the TUI dashboard")
}

func getDatabasePath() string {
	home, _ := os.UserHomeDir()
	
	// Create the directory path: /home/user/.local/share/boring-ci
	dir := filepath.Join(home, ".local", "share", "boring-ci")
	
	// Create the folder if it doesn't exist (mkdir -p)
	// 0700 means only the daemon user can read/write this folder
	os.MkdirAll(dir, 0700) 
	
	return filepath.Join(dir, "boring.db")
}

func createSchema(db *sql.DB) error {
	// SQL statement for the builds table
	// Using 'IF NOT EXISTS' is the industry standard for auto-setup
	schema := `
	CREATE TABLE IF NOT EXISTS builds (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	repo TEXT NOT NULL,
	branch TEXT NOT NULL,
	commit_hash TEXT NOT NULL,
	pipeline TEXT NOT NULL,
	status TEXT DEFAULT 'pending',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	started_at DATETIME,
	finished_at DATETIME
	);`

	_, err := db.Exec(schema)
	return err
}

func daemonMode() {
	var err error

	db, err = sql.Open("sqlite3", getDatabasePath())

	if err != nil {
    fmt.Printf("Error opening DB: %v\n", err)

    os.Exit(1)
  }
  
	defer db.Close()

	err = createSchema(db)

	if err != nil {
		fmt.Printf("Error creating schema: %v\n", err)

		os.Exit(1)
	}

	socketPath := "/tmp/boring.sock"

	os.Remove(socketPath)

	l, _ := net.Listen("unix", "/tmp/boring.sock")

	os.Chmod(socketPath, 0660)

	fmt.Println("Server is up and accepting builds\n");

	for {
		conn, _ := l.Accept();

		go handleCli(conn);
	}
}

func handleCli(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	writer := bufio.NewWriter(conn)

	for scanner.Scan() {
		command := scanner.Text()

		if command == "PING" {
			writer.WriteString("PONG\n")
			writer.Flush()

			continue
		}

		parts := strings.Fields(command)

		switch parts[0] {
		case "trigger":
			err := triggerBuild(parts[1], parts[2], parts[3], parts[4])

			if err != nil {
				msg := fmt.Sprintf("Error: %s\n", err)

				writer.WriteString(msg)
			} else {
				writer.WriteString("Success!\n")
			}

			writer.Flush()
		case "watch":
			encoder := json.NewEncoder(conn)

			for {
				rows, _ := db.Query("SELECT id, repo, branch, status, commit_hash, created_at, started_at, finished_at FROM builds ORDER BY id DESC LIMIT 15")

				var list []Build

				for rows.Next() {
					var b Build

					rows.Scan(&b.ID, &b.Repo, &b.Branch, &b.Status, &b.Commit, &b.CreatedAt, &b.StartedAt, &b.FinishedAt)
					list = append(list, b)
				}

				rows.Close()

				if err := encoder.Encode(list); err != nil {
					fmt.Printf("Error from SQL: %s", err);
				}

				time.Sleep(2 * time.Second)
			}
		default:
			writer.WriteString("Unknown command\n");
		}
	}
}

func callDaemon(msg string) {
    conn, err := net.Dial("unix", "/tmp/boring.sock")

    if err != nil {
        fmt.Println("Could not find boring-ci server. Is it running?")
        return
    }

    defer conn.Close()

    fmt.Fprintln(conn, msg)

    scanner := bufio.NewScanner(conn)

    if scanner.Scan() {
      fmt.Println(scanner.Text())
    }
}

func triggerBuild(repo string, branch string, commit string, pipeline string) error {
	home, err := os.UserHomeDir()

	if err != nil {
		return errors.New("Can't find home dir")
	}

	path := filepath.Join(home, ".config", "boring-ci", "repos.json")

	data, err := os.ReadFile(path)

	if err != nil {
		return fmt.Errorf("Can't find file %s", path)
	}

	repos := make(map[string]string)

	err = json.Unmarshal(data, &repos)

	if err != nil {
		return errors.New("Error in parsing repos.json, make sure it's valid JSON and follows the { string: string, ... } format.");
	}

	for name, _ := range repos {
		if name == repo {
			fmt.Printf("New build scheduled: repo %s (%s pipeline), branch %s, commit: %s", repo, pipeline, branch, commit) 

			saveBuild(repo, branch, commit, pipeline);

			return nil
		}
	}

	return errors.New("Could not find that repo in repos.json");
}

func saveBuild(repo string, branch string, commit string, pipeline string) error {
	query := `INSERT INTO builds (repo, branch, commit_hash, pipeline) VALUES (?, ?, ?, ?)`

	result, err := db.Exec(query, repo, branch, commit, pipeline)

	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	id, _ := result.LastInsertId()

	fmt.Printf("Recorded build #%d in database\n", id)

	return nil
}

func runDashboardCLI() {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		fmt.Println("Daemon offline"); return
	}
	defer conn.Close()

	fmt.Print("\033[?25l")       // Hide cursor
  defer fmt.Print("\033[?25h") // Show cursor when we exit

	// Enter Raw Mode
	fd := int(os.Stdin.Fd())
	oldState, _ := term.MakeRaw(fd)
	defer term.Restore(fd, oldState)

	state := &DashboardState{}
	fmt.Fprintln(conn, "watch") // Request persistent stream

	// Reader: Listen for JSON updates from Daemon
	go func() {
		dec := json.NewDecoder(conn)
		for {
			var fresh []Build
			if err := dec.Decode(&fresh); err != nil { return }
			state.Mu.Lock()
			state.Builds = fresh
			state.render()
			state.Mu.Unlock()
		}
	}()

	// Input: Handle j/k navigation
	reader := bufio.NewReader(os.Stdin)
	// Inside your main input loop
	for {
    char, _, _ := reader.ReadRune()
    state.Mu.Lock()
    
    if char == 'q' || char == '\x03' { return }

    if state.ActiveView == ViewDashboard {
        switch char {
        case 'j': state.SelectedIndex++
        case 'k': state.SelectedIndex--
        case '\r': state.ActiveView = ViewLogs // Hit Enter
        }
    } else if state.ActiveView == ViewLogs {
        switch char {
        case 'q', '\x1b': state.ActiveView = ViewDashboard // 'q' or Esc to go back
        case 'j': state.LogScrollIdx++
        case 'k': state.LogScrollIdx--
        }
    }
    
    state.render() // One render function to rule them all
    state.Mu.Unlock()
	}
}

func (s *DashboardState) render() {
    width, height, _ := term.GetSize(int(os.Stdout.Fd()))
    var out strings.Builder
    out.WriteString("\033[H\033[2J") // Clear

    if s.ActiveView == ViewDashboard {
        s.renderDashboardView(&out, width)
    } else {
        s.renderLogView(&out, width, height)
    }

    os.Stdout.WriteString(out.String())
}

func (b Build) Elapsed() string {
	// If the build hasn't started yet, time is 0
	if b.StartedAt == nil {
		return "0s"
	}

	var end time.Time
	if b.FinishedAt != nil {
		// Build is finished, use the recorded end time
		end = *b.FinishedAt
	} else {
		// Build is still running, use current time for live count
		end = time.Now()
	}

	duration := end.Sub(*b.StartedAt)

	// Format: Under a minute shows seconds, over shows minutes/seconds
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}
	
	return fmt.Sprintf("%dm%ds", int(duration.Minutes()), int(duration.Seconds())%60)
}

func (s *DashboardState) renderDashboardView(out *strings.Builder, width int) {
	// Title Bar
	title := " BORING CI "
	nav := "(j/k: Nav | Enter: Logs | q: Quit) "
	padTitle := width - len(title) - len(nav)
	if padTitle < 0 { padTitle = 0 }
	out.WriteString("\033[44;37;1m" + title + strings.Repeat(" ", padTitle) + nav + "\033[0m\r\n\r\n")

	// Header
	header := fmt.Sprintf("%-4s %-15s %-15s %-10s %-12s %-20s %-8s", 
		"ID", "REPO", "BRANCH", "COMMIT", "STATUS", "CREATED", "TIME")
	out.WriteString(fmt.Sprintf("\033[1m%-*s\033[0m\r\n", width, header))
	out.WriteString(strings.Repeat("─", width) + "\r\n")

	for i, b := range s.Builds {
		repo := b.Repo; if len(repo) > 14 { repo = repo[:11] + "..." }
		branch := b.Branch; if len(branch) > 14 { branch = branch[:11] + "..." }
		commit := b.Commit; if len(commit) > 8 { commit = commit[:8] }

		line := fmt.Sprintf("%-4d %-15s %-15s %-10s %-12s %-20s %-8s",
			b.ID, repo, branch, commit, strings.ToUpper(b.Status), b.CreatedAt, b.Elapsed())

		if i == s.SelectedIndex {
			padding := ""
			if len(line) < width { padding = strings.Repeat(" ", width-len(line)) }
			out.WriteString("\033[46;30m" + line + padding + "\033[0m\r\n")
		} else {
			out.WriteString(line + "\r\n")
		}
	}
}

func (s *DashboardState) renderLogView(out *strings.Builder, width, height int) {
	// 1. Get the currently selected build for metadata
	if len(s.Builds) == 0 || s.SelectedIndex >= len(s.Builds) {
		out.WriteString("No build selected\r\n")
		return
	}
	b := s.Builds[s.SelectedIndex]

	// 2. Prepare Header Strings
	// Left side: ID, Repo, Branch, Commit, Pipeline
	leftHeader := fmt.Sprintf(" BUILD #%d: %s | %s | %s | %s ", 
		b.ID, b.Repo, b.Branch, b.Commit, b.Pipeline)
	
	// Right side: Created and Elapsed
	rightHeader := fmt.Sprintf(" CREATED: %s | ELAPSED: %s ", 
		b.CreatedAt, b.Elapsed())

	// Calculate padding to push metadata to the right
	padWidth := width - len(leftHeader) - len(rightHeader)
	if padWidth < 0 { padWidth = 0 }

	// 3. Draw the Blue Bar (Full Width)
	out.WriteString("\033[44;37;1m" + leftHeader + strings.Repeat(" ", padWidth) + rightHeader + "\033[0m\r\n\r\n")

	// 4. Draw the Split View (Sidebar + Logs)
	sidebarW := width / 4
	logW := width - sidebarW - 2
	menu := []string{"step-1.sh", "step-2.sh", "step-3.sh"}

	for i := 0; i < height-5; i++ {
		// Sidebar Item
		sideTxt := ""
		if i < len(menu) {
			if i == s.LogScrollIdx {
				sideTxt = fmt.Sprintf("\033[46;30m%-*s\033[0m", sidebarW, " "+menu[i])
			} else {
				sideTxt = fmt.Sprintf("%-*s", sidebarW, " "+menu[i])
			}
		} else {
			sideTxt = strings.Repeat(" ", sidebarW)
		}

		// Vertical Divider
		divider := "│"

		// Logs (Simulated - or from your log buffer)
		logLine := ""
		if i < len(s.Logs) { // Assuming you have a s.Logs []string
			logLine = s.Logs[i]
			if len(logLine) > logW { logLine = logLine[:logW] }
		}

		out.WriteString(sideTxt + divider + logLine + "\r\n")
	}
}
