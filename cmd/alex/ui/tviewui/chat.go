package tviewui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"alex/cmd/alex/ui/eventhub"
	"alex/cmd/alex/ui/state"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
	"alex/internal/config"
	"alex/internal/mcp"
	"alex/internal/tools/builtin"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type paneSearch struct {
	query   string
	matches []int
	current int
}

// Config configures the chat TUI.
type MCPRegistry interface {
	ListServers() []*mcp.ServerInstance
	RestartServer(name string) error
}

// Coordinator exposes the coordinator functionality required by the chat UI.
type Coordinator interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener ports.EventListener) (*ports.TaskResult, error)
	ListSessions(ctx context.Context) ([]string, error)
	GetSession(ctx context.Context, id string) (*ports.Session, error)
	GetCostTracker() ports.CostTracker
}

type Config struct {
	Coordinator      Coordinator
	Store            *state.Store
	Hub              *eventhub.Hub
	Registry         MCPRegistry
	Verbose          bool
	CostTracker      ports.CostTracker
	FollowTranscript *bool
	FollowStream     *bool
	FollowSaver      func(bool, bool) (string, error)
}

// ChatUI manages the interactive tview-based chat shell.
type ChatUI struct {
	coordinator Coordinator
	store       *state.Store
	hub         *eventhub.Hub
	ownsHub     bool
	registry    MCPRegistry
	costTracker ports.CostTracker

	app         *tview.Application
	transcript  *tview.TextView
	liveStream  *tview.TextView
	tools       *tview.TextView
	subagents   *tview.TextView
	mcp         *tview.TextView
	status      *tview.TextView
	input       *tview.InputField
	layout      *tview.Flex
	pages       *tview.Pages
	helpModal   *tview.Modal
	searchModal *tview.Flex
	searchInput *tview.InputField

	updates chan state.Update

	ctx    context.Context
	cancel context.CancelFunc

	sessionMu sync.Mutex
	sessionID string

	pending atomic.Int32

	focusables       []tview.Primitive
	focusIndex       int
	lastFocus        tview.Primitive
	helpVisible      bool
	searchVisible    bool
	searchTarget     *tview.TextView
	searchReturn     tview.Primitive
	followTranscript atomic.Bool
	followStream     atomic.Bool

	defaultFollowTranscript bool
	defaultFollowStream     bool

	searchStates  map[*tview.TextView]*paneSearch
	searchSummary atomic.Value
	spinnerFrame  atomic.Int32

	mcpKnown map[string]state.MCPStatus

	outputCtx *types.OutputContext
	verbose   bool

	costRefreshPending atomic.Bool

	saveFollowPreferences func(bool, bool) (string, error)
}

// NewChatUI builds a ChatUI instance from the provided config.
func NewChatUI(cfg Config) (*ChatUI, error) {
	if cfg.Coordinator == nil {
		return nil, fmt.Errorf("coordinator is required")
	}

	store := cfg.Store
	if store == nil {
		store = state.NewStore()
	}

	hub := cfg.Hub
	ownsHub := false
	if hub == nil {
		hub = eventhub.NewHub()
		ownsHub = true
	}

	baseCtx := context.Background()
	outCtx := &types.OutputContext{
		Level:   types.LevelCore,
		AgentID: "core",
		Verbose: cfg.Verbose,
	}
	baseCtx = types.WithOutputContext(baseCtx, outCtx)
	ctx, cancel := context.WithCancel(baseCtx)

	tracker := cfg.CostTracker
	if tracker == nil {
		tracker = cfg.Coordinator.GetCostTracker()
	}

	followTranscript := true
	if cfg.FollowTranscript != nil {
		followTranscript = *cfg.FollowTranscript
	}
	followStream := true
	if cfg.FollowStream != nil {
		followStream = *cfg.FollowStream
	}

	saver := cfg.FollowSaver
	if saver == nil {
		saver = func(transcript, stream bool) (string, error) {
			return config.SaveFollowPreferences(transcript, stream)
		}
	}

	ui := &ChatUI{
		coordinator:             cfg.Coordinator,
		store:                   store,
		hub:                     hub,
		ownsHub:                 ownsHub,
		registry:                cfg.Registry,
		costTracker:             tracker,
		app:                     tview.NewApplication(),
		transcript:              buildTranscriptView(),
		liveStream:              buildLiveStreamView(),
		tools:                   buildToolRunsView(),
		subagents:               buildSubagentView(),
		mcp:                     buildMCPView(),
		status:                  buildStatusView(),
		input:                   buildInputField(),
		layout:                  tview.NewFlex().SetDirection(tview.FlexRow),
		ctx:                     ctx,
		cancel:                  cancel,
		verbose:                 cfg.Verbose,
		mcpKnown:                make(map[string]state.MCPStatus),
		searchStates:            make(map[*tview.TextView]*paneSearch),
		outputCtx:               outCtx,
		defaultFollowTranscript: followTranscript,
		defaultFollowStream:     followStream,
		saveFollowPreferences:   saver,
	}

	ui.searchSummary.Store("")
	ui.restoreFollowDefaults()
	ui.composeLayout()
	ui.configureInteraction()
	return ui, nil
}

// Run starts the UI event loop.
func (ui *ChatUI) Run() error {
	if ui.app == nil {
		return fmt.Errorf("application not initialized")
	}

	ui.start()
	defer ui.shutdown()

	return ui.app.Run()
}

func (ui *ChatUI) start() {
	ui.updates = ui.hub.Subscribe(0)
	ui.input.SetDoneFunc(ui.onInputDone)
	ui.bootstrapSessionHistory()
	ui.renderSnapshot()

	go ui.consumeUpdates()
	go ui.observeMCP()
	go ui.animateMCP()
}

func (ui *ChatUI) shutdown() {
	if ui.cancel != nil {
		ui.cancel()
	}
	if ui.updates != nil {
		ui.hub.Unsubscribe(ui.updates)
	}
	if ui.ownsHub {
		ui.hub.Close()
	}
}

func (ui *ChatUI) composeLayout() {
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[::b]Alex Interactive Chat[-]  •  Press Ctrl+C or type /quit to exit")
	header.SetBorder(false)

	leftColumn := tview.NewFlex().SetDirection(tview.FlexRow)
	leftColumn.AddItem(ui.transcript, 0, 3, false)
	leftColumn.AddItem(ui.liveStream, 10, 1, false)

	rightColumn := tview.NewFlex().SetDirection(tview.FlexRow)
	rightColumn.AddItem(ui.subagents, 0, 1, false)
	rightColumn.AddItem(ui.tools, 0, 1, false)
	rightColumn.AddItem(ui.mcp, 0, 1, false)

	body := tview.NewFlex().SetDirection(tview.FlexColumn)
	body.AddItem(leftColumn, 0, 2, false)
	body.AddItem(rightColumn, 0, 1, false)

	ui.input.SetLabel("» ")
	ui.input.SetFieldBackgroundColor(tcell.ColorDefault)

	ui.layout.
		AddItem(header, 1, 0, false).
		AddItem(body, 0, 1, true).
		AddItem(ui.status, 1, 0, false).
		AddItem(ui.input, 3, 0, true)

	ui.pages = tview.NewPages()
	ui.pages.AddPage("main", ui.layout, true, true)

	ui.app.SetRoot(ui.pages, true)
	ui.app.SetFocus(ui.input)
}

func (ui *ChatUI) configureInteraction() {
	if ui.app == nil {
		return
	}

	ui.configureTextView(ui.transcript, &ui.followTranscript)
	ui.configureTextView(ui.liveStream, &ui.followStream)
	ui.configureTextView(ui.tools, nil)
	ui.configureTextView(ui.subagents, nil)
	ui.configureTextView(ui.mcp, nil)

	ui.focusables = []tview.Primitive{ui.transcript, ui.liveStream, ui.tools, ui.subagents, ui.mcp, ui.input}
	ui.focusIndex = len(ui.focusables) - 1

	ui.app.SetInputCapture(ui.globalInputCapture)

	if ui.helpModal == nil {
		ui.helpModal = ui.buildHelpModal()
	}
	if ui.pages != nil && ui.helpModal != nil {
		ui.pages.AddPage("help", ui.helpModal, true, false)
	}

	if ui.searchModal == nil {
		ui.searchModal, ui.searchInput = ui.buildSearchModal()
	}
	if ui.pages != nil && ui.searchModal != nil {
		ui.pages.AddPage("search", ui.searchModal, true, false)
	}

	if ui.input != nil {
		ui.setFocus(ui.input)
	}
}

func (ui *ChatUI) configureTextView(view *tview.TextView, follow *atomic.Bool) {
	if view == nil {
		return
	}
	view.SetScrollable(true)
	view.SetRegions(true)
	if follow != nil {
		view.SetChangedFunc(func() {
			if follow.Load() {
				view.ScrollToEnd()
			}
		})
	} else {
		view.SetChangedFunc(func() {
			view.ScrollToBeginning()
		})
	}
	view.SetInputCapture(ui.textViewInputHandler(view, follow))
}

func (ui *ChatUI) globalInputCapture(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return nil
	}

	switch {
	case event.Key() == tcell.KeyTAB && !ui.helpVisible:
		ui.focusNext()
		return nil
	case event.Key() == tcell.KeyBacktab && !ui.helpVisible:
		ui.focusPrevious()
		return nil
	case event.Key() == tcell.KeyCtrlL:
		ui.followTranscript.Store(true)
		ui.followStream.Store(true)
		ui.queueFollowToEnd()
		return nil
	case event.Key() == tcell.KeyRune && event.Rune() == '?':
		ui.toggleHelp()
		return nil
	}

	return event
}

func (ui *ChatUI) textViewInputHandler(view *tview.TextView, follow *atomic.Bool) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event == nil {
			return nil
		}

		switch event.Key() {
		case tcell.KeyPgUp:
			if follow != nil {
				follow.Store(false)
			}
			ui.scrollTextView(view, -ui.pageScrollAmount(view))
			return nil
		case tcell.KeyPgDn:
			if follow != nil {
				follow.Store(false)
			}
			ui.scrollTextView(view, ui.pageScrollAmount(view))
			return nil
		case tcell.KeyUp:
			if follow != nil {
				follow.Store(false)
			}
			ui.scrollTextView(view, -1)
			return nil
		case tcell.KeyDown:
			if follow != nil {
				follow.Store(false)
			}
			ui.scrollTextView(view, 1)
			return nil
		case tcell.KeyHome:
			if follow != nil {
				follow.Store(false)
			}
			view.ScrollToBeginning()
			return nil
		case tcell.KeyEnd:
			if follow != nil {
				follow.Store(true)
			}
			view.ScrollToEnd()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case '/':
				if follow != nil {
					follow.Store(false)
				}
				ui.openSearch(view)
				return nil
			case 'n':
				if follow != nil {
					follow.Store(false)
				}
				ui.advanceSearch(view, 1)
				return nil
			case 'N':
				if follow != nil {
					follow.Store(false)
				}
				ui.advanceSearch(view, -1)
				return nil
			}
		}

		return event
	}
}

func (ui *ChatUI) pageScrollAmount(view *tview.TextView) int {
	if view == nil {
		return 0
	}
	_, _, _, height := view.GetInnerRect()
	if height <= 1 {
		return 1
	}
	return height - 1
}

func (ui *ChatUI) scrollTextView(view *tview.TextView, delta int) {
	if view == nil || delta == 0 {
		return
	}
	row, col := view.GetScrollOffset()
	row += delta
	if row < 0 {
		row = 0
	}
	view.ScrollTo(row, col)
}

func (ui *ChatUI) focusNext() {
	if len(ui.focusables) == 0 {
		return
	}
	ui.focusIndex = (ui.focusIndex + 1) % len(ui.focusables)
	ui.setFocus(ui.focusables[ui.focusIndex])
}

func (ui *ChatUI) focusPrevious() {
	if len(ui.focusables) == 0 {
		return
	}
	ui.focusIndex--
	if ui.focusIndex < 0 {
		ui.focusIndex = len(ui.focusables) - 1
	}
	ui.setFocus(ui.focusables[ui.focusIndex])
}

func (ui *ChatUI) setFocus(p tview.Primitive) {
	if p == nil || ui.app == nil {
		return
	}
	ui.app.SetFocus(p)
	for i, candidate := range ui.focusables {
		if candidate == p {
			ui.focusIndex = i
			break
		}
	}
}

func (ui *ChatUI) toggleHelp() {
	if ui.helpVisible {
		ui.hideHelp()
	} else {
		ui.showHelp()
	}
}

func (ui *ChatUI) showHelp() {
	if ui.helpModal == nil || ui.pages == nil || ui.app == nil {
		return
	}
	ui.helpVisible = true
	ui.lastFocus = ui.app.GetFocus()
	ui.app.QueueUpdateDraw(func() {
		ui.pages.ShowPage("help")
		ui.app.SetFocus(ui.helpModal)
	})
}

func (ui *ChatUI) hideHelp() {
	if ui.helpModal == nil || ui.pages == nil || ui.app == nil {
		return
	}
	ui.helpVisible = false
	ui.app.QueueUpdateDraw(func() {
		ui.pages.HidePage("help")
		if ui.lastFocus != nil {
			ui.app.SetFocus(ui.lastFocus)
		} else if len(ui.focusables) > 0 {
			ui.setFocus(ui.focusables[len(ui.focusables)-1])
		}
	})
}

func (ui *ChatUI) buildHelpModal() *tview.Modal {
	text := strings.Join([]string{
		"[::b]Alex Chat Shortcuts[-]",
		"",
		"[white]?[-] Toggle help",
		"[white]Tab[-]/[white]Shift+Tab[-] Cycle focus",
		"[white]PgUp/PgDn[-] Scroll panes",
		"[white]End[-] Resume follow",
		"[white]Ctrl+L[-] Jump to latest",
		"[white]/[-] Search current pane",
		"[white]n/N[-] Next or previous match",
		"[white]/quit[-] Exit session",
		"[white]Ctrl+C[-] Immediate exit",
	}, "\n")

	modal := tview.NewModal().
		SetText(text).
		SetBackgroundColor(tcell.ColorDefault).
		AddButtons([]string{"Close"})

	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ui.hideHelp()
	})
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event == nil {
			return nil
		}
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyEnter:
			ui.hideHelp()
			return nil
		case tcell.KeyRune:
			if event.Rune() == '?' {
				ui.hideHelp()
				return nil
			}
		}
		return event
	})

	return modal
}

func (ui *ChatUI) buildSearchModal() (*tview.Flex, *tview.InputField) {
	input := tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(40)
	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			ui.commitSearch()
		case tcell.KeyEscape:
			ui.closeSearch()
		}
	})

	hint := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[gray]Enter to jump  Esc cancel  n/N to navigate[-]")

	modal := tview.NewFlex().SetDirection(tview.FlexRow)
	modal.SetBorder(true).SetTitle("Search")
	modal.AddItem(input, 0, 1, true)
	modal.AddItem(hint, 2, 0, false)
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event == nil {
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			ui.closeSearch()
			return nil
		}
		return event
	})

	return modal, input
}

func (ui *ChatUI) queueFollowToEnd() {
	if ui.app == nil {
		return
	}
	ui.app.QueueUpdateDraw(func() {
		if ui.transcript != nil {
			ui.transcript.ScrollToEnd()
		}
		if ui.liveStream != nil {
			ui.liveStream.ScrollToEnd()
		}
	})
}

func (ui *ChatUI) restoreFollowDefaults() {
	ui.followTranscript.Store(ui.defaultFollowTranscript)
	ui.followStream.Store(ui.defaultFollowStream)
}

func (ui *ChatUI) openSearch(target *tview.TextView) {
	if target == nil || ui.app == nil || ui.pages == nil || ui.searchModal == nil {
		return
	}
	state := ui.ensureSearchState(target)
	if ui.searchInput != nil && state != nil {
		ui.searchInput.SetText(state.query)
	}
	ui.searchTarget = target
	ui.searchVisible = true
	ui.searchReturn = ui.app.GetFocus()
	ui.app.QueueUpdateDraw(func() {
		ui.pages.ShowPage("search")
		if ui.searchInput != nil {
			ui.app.SetFocus(ui.searchInput)
		}
	})
}

func (ui *ChatUI) commitSearch() {
	var query string
	if ui.searchInput != nil {
		query = ui.searchInput.GetText()
	}
	ui.performSearch(ui.searchTarget, query)
	ui.closeSearch()
}

func (ui *ChatUI) closeSearch() {
	if ui.app == nil || ui.pages == nil {
		ui.searchTarget = nil
		ui.searchVisible = false
		ui.searchReturn = nil
		return
	}
	if !ui.searchVisible {
		ui.searchTarget = nil
		ui.searchReturn = nil
		return
	}
	returnFocus := ui.searchReturn
	ui.searchVisible = false
	ui.searchTarget = nil
	ui.searchReturn = nil
	ui.app.QueueUpdateDraw(func() {
		ui.pages.HidePage("search")
		if returnFocus != nil {
			ui.app.SetFocus(returnFocus)
		}
	})
}

func (ui *ChatUI) ensureSearchState(view *tview.TextView) *paneSearch {
	if view == nil {
		return nil
	}
	state, ok := ui.searchStates[view]
	if !ok {
		state = &paneSearch{current: -1}
		ui.searchStates[view] = state
	}
	return state
}

func (ui *ChatUI) performSearch(view *tview.TextView, query string) {
	state := ui.ensureSearchState(view)
	trimmed := strings.TrimSpace(query)
	if state == nil {
		ui.updateSearchSummary("", nil, -1)
		ui.refreshStatusFromStore()
		return
	}
	state.query = trimmed
	state.current = -1
	state.matches = nil
	if trimmed == "" || view == nil {
		ui.updateSearchSummary("", nil, -1)
		ui.refreshStatusFromStore()
		return
	}

	text := view.GetText(false)
	state.matches = findMatchingLines(text, trimmed)
	if len(state.matches) == 0 {
		ui.updateSearchSummary(trimmed, nil, -1)
		ui.refreshStatusFromStore()
		return
	}

	state.current = 0
	ui.scrollToLine(view, state.matches[state.current])
	ui.updateSearchSummary(trimmed, state.matches, state.current)
	ui.refreshStatusFromStore()
}

func (ui *ChatUI) advanceSearch(view *tview.TextView, delta int) {
	if view == nil || delta == 0 {
		return
	}
	state, ok := ui.searchStates[view]
	if !ok || strings.TrimSpace(state.query) == "" {
		return
	}

	ui.refreshMatches(view, state)
	if len(state.matches) == 0 {
		ui.updateSearchSummary(state.query, nil, -1)
		ui.refreshStatusFromStore()
		return
	}

	state.current = (state.current + delta) % len(state.matches)
	if state.current < 0 {
		state.current += len(state.matches)
	}

	ui.scrollToLine(view, state.matches[state.current])
	ui.updateSearchSummary(state.query, state.matches, state.current)
	ui.refreshStatusFromStore()
}

func (ui *ChatUI) refreshMatches(view *tview.TextView, state *paneSearch) {
	if view == nil || state == nil {
		return
	}
	if strings.TrimSpace(state.query) == "" {
		state.matches = nil
		state.current = -1
		return
	}
	text := view.GetText(false)
	matches := findMatchingLines(text, state.query)
	state.matches = matches
	if len(matches) == 0 {
		state.current = -1
		return
	}
	if state.current < 0 || state.current >= len(matches) {
		state.current = 0
	}
}

func (ui *ChatUI) scrollToLine(view *tview.TextView, line int) {
	if view == nil || line < 0 {
		return
	}
	if ui.app == nil {
		view.ScrollTo(line, 0)
		return
	}
	ui.app.QueueUpdateDraw(func() {
		view.ScrollTo(line, 0)
	})
}

func (ui *ChatUI) updateSearchSummary(query string, matches []int, index int) {
	summary := ""
	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		if len(matches) == 0 {
			summary = fmt.Sprintf("Search \"%s\": no matches", trimmed)
		} else {
			if index < 0 {
				index = 0
			}
			summary = fmt.Sprintf("Search \"%s\": %d/%d", trimmed, index+1, len(matches))
		}
	}
	ui.searchSummary.Store(summary)
}

func (ui *ChatUI) refreshStatusFromStore() {
	if ui.app == nil {
		return
	}
	snapshot := ui.store.Snapshot()
	status := renderStatus(int(ui.pending.Load()), snapshot, ui.activeSessionID(), ui.currentSearchSummary(), ui.verbose)
	ui.app.QueueUpdateDraw(func() {
		ui.status.SetText(status)
	})
}

func (ui *ChatUI) currentSearchSummary() string {
	value := ui.searchSummary.Load()
	if value == nil {
		return ""
	}
	summary, _ := value.(string)
	return summary
}

func (ui *ChatUI) activeSessionID() string {
	ui.sessionMu.Lock()
	defer ui.sessionMu.Unlock()
	return ui.sessionID
}

func (ui *ChatUI) setSessionID(id string) {
	ui.sessionMu.Lock()
	defer ui.sessionMu.Unlock()
	ui.sessionID = id
}

func (ui *ChatUI) bootstrapSessionHistory() {
	if ui.coordinator == nil {
		return
	}

	session, err := ui.loadLatestSession(ui.ctx)
	if err != nil {
		ui.appendSystemMessage(fmt.Sprintf("Failed to load previous session: %v", err))
		return
	}

	if session == nil {
		return
	}

	ui.applySessionSnapshot(session)
	ui.appendSystemMessage(fmt.Sprintf("Resumed session %s (%d messages)", session.ID, len(session.Messages)))
}

func (ui *ChatUI) loadLatestSession(ctx context.Context) (*ports.Session, error) {
	ids, err := ui.coordinator.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	var latest *ports.Session
	var latestTime time.Time

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		session, err := ui.coordinator.GetSession(ctx, id)
		if err != nil || session == nil {
			continue
		}
		ts := session.UpdatedAt
		if ts.IsZero() {
			ts = session.CreatedAt
		}
		if latest == nil || ts.After(latestTime) {
			latest = session
			latestTime = ts
		}
	}

	return latest, nil
}

func buildTranscriptView() *tview.TextView {
	view := tview.NewTextView()
	view.SetBorder(true)
	view.SetTitle("Conversation")
	view.SetDynamicColors(true)
	view.SetWrap(true)
	view.SetWordWrap(true)
	return view
}

func buildLiveStreamView() *tview.TextView {
	view := tview.NewTextView()
	view.SetBorder(true)
	view.SetTitle("Live Stream")
	view.SetDynamicColors(true)
	view.SetWrap(true)
	view.SetWordWrap(true)
	return view
}

func buildToolRunsView() *tview.TextView {
	view := tview.NewTextView()
	view.SetBorder(true)
	view.SetTitle("Tools")
	view.SetDynamicColors(true)
	view.SetWrap(true)
	view.SetWordWrap(true)
	return view
}

func buildSubagentView() *tview.TextView {
	view := tview.NewTextView()
	view.SetBorder(true)
	view.SetTitle("Subagents")
	view.SetDynamicColors(true)
	view.SetWrap(true)
	view.SetWordWrap(true)
	return view
}

func buildMCPView() *tview.TextView {
	view := tview.NewTextView()
	view.SetBorder(true)
	view.SetTitle("MCP Servers")
	view.SetDynamicColors(true)
	view.SetWrap(true)
	view.SetWordWrap(true)
	return view
}

func buildStatusView() *tview.TextView {
	view := tview.NewTextView()
	view.SetDynamicColors(true)
	view.SetBorder(false)
	return view
}

func buildInputField() *tview.InputField {
	field := tview.NewInputField()
	field.SetPlaceholder("Ask Alex anything…")
	field.SetFieldWidth(0)
	return field
}

func (ui *ChatUI) consumeUpdates() {
	for update := range ui.updates {
		ui.store.Apply(update)
		switch update.(type) {
		case state.MetricsDelta:
			ui.queueCostRefresh()
		}
		ui.renderSnapshot()
	}
}

func (ui *ChatUI) queueCostRefresh() {
	if ui.costTracker == nil {
		return
	}

	sessionID := strings.TrimSpace(ui.activeSessionID())
	if sessionID == "" {
		return
	}

	if !ui.costRefreshPending.CompareAndSwap(false, true) {
		return
	}

	go ui.refreshCostSummary(sessionID)
}

func (ui *ChatUI) applyCostSummaryMetrics(summary *ports.CostSummary) {
	if summary == nil {
		return
	}

	costByModel := make(map[string]float64, len(summary.ByModel))
	for model, cost := range summary.ByModel {
		costByModel[model] = cost
	}

	timestamp := summary.EndTime
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	ui.store.Apply(state.MetricsCostSummary{
		TotalCost:   summary.TotalCost,
		CostByModel: costByModel,
		Timestamp:   timestamp,
	})
}

func (ui *ChatUI) refreshCostSummary(sessionID string) {
	defer ui.costRefreshPending.Store(false)

	if ui.costTracker == nil || sessionID == "" {
		return
	}

	summary, err := ui.costTracker.GetSessionCost(ui.ctx, sessionID)
	if err != nil || summary == nil {
		return
	}

	ui.applyCostSummaryMetrics(summary)
	ui.renderSnapshot()
}

func (ui *ChatUI) onInputDone(key tcell.Key) {
	if key != tcell.KeyEnter {
		if key == tcell.KeyEscape {
			ui.app.Stop()
		}
		return
	}

	text := strings.TrimSpace(ui.input.GetText())
	if text == "" {
		return
	}

	ui.input.SetText("")

	if strings.HasPrefix(text, "/") {
		if ui.handleSlashCommand(text) {
			return
		}
	}

	ui.appendUserMessage(text)
	go ui.executeTask(text)
}

func (ui *ChatUI) handleSlashCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return true
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return true
	}

	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/quit", "/exit":
		ui.app.Stop()
		return true
	case "/new":
		go ui.startNewSessionCommand()
		return true
	case "/sessions":
		go ui.listSessionsCommand()
		return true
	case "/load":
		if len(parts) < 2 {
			ui.appendSystemMessage("Usage: /load <session_id>")
			ui.renderSnapshot()
			return true
		}
		go ui.loadSessionCommand(strings.Join(parts[1:], " "))
		return true
	case "/mcp":
		go ui.mcpCommand(parts[1:])
		return true
	case "/cost":
		go ui.costCommand(parts[1:])
		return true
	case "/export":
		var target string
		if len(parts) > 1 {
			target = strings.Join(parts[1:], " ")
		}
		go ui.exportTranscriptCommand(target)
		return true
	case "/verbose":
		go ui.verboseCommand(parts[1:])
		return true
	case "/follow":
		go ui.followCommand(parts[1:])
		return true
	default:
		ui.appendSystemMessage(fmt.Sprintf("Unknown command: %s", parts[0]))
		ui.renderSnapshot()
		return true
	}
}

func (ui *ChatUI) appendUserMessage(text string) {
	ui.store.Apply(state.MessageAppend{Message: state.ChatMessage{
		Role:      state.RoleUser,
		AgentID:   "user",
		Content:   text,
		CreatedAt: time.Now(),
	}})
	ui.renderSnapshot()
}

func (ui *ChatUI) appendSystemMessage(text string) {
	ui.store.Apply(state.MessageAppend{Message: state.ChatMessage{
		Role:      state.RoleSystem,
		AgentID:   "system",
		Content:   text,
		CreatedAt: time.Now(),
	}})
}

func (ui *ChatUI) applySessionSnapshot(session *ports.Session) {
	ui.setSessionID("")
	ui.store.Apply(state.Reset{})
	ui.restoreFollowDefaults()

	if session == nil {
		return
	}

	ui.setSessionID(session.ID)
	for _, msg := range session.Messages {
		for _, chat := range convertSessionMessage(msg) {
			ui.store.Apply(state.MessageAppend{Message: chat})
		}
	}

	ui.queueCostRefresh()
}

func (ui *ChatUI) startNewSessionCommand() {
	if ui.coordinator == nil {
		ui.appendSystemMessage("Coordinator unavailable; cannot start a new session.")
		ui.renderSnapshot()
		return
	}

	session, err := ui.coordinator.GetSession(ui.ctx, "")
	if err != nil {
		ui.appendSystemMessage(fmt.Sprintf("Failed to start new session: %v", err))
		ui.renderSnapshot()
		return
	}

	ui.applySessionSnapshot(session)
	ui.appendSystemMessage(fmt.Sprintf("Started new session %s", session.ID))
	ui.renderSnapshot()
}

func (ui *ChatUI) listSessionsCommand() {
	if ui.coordinator == nil {
		ui.appendSystemMessage("Coordinator unavailable; cannot list sessions.")
		ui.renderSnapshot()
		return
	}

	ids, err := ui.coordinator.ListSessions(ui.ctx)
	if err != nil {
		ui.appendSystemMessage(fmt.Sprintf("Failed to list sessions: %v", err))
		ui.renderSnapshot()
		return
	}

	if len(ids) == 0 {
		ui.appendSystemMessage("No saved sessions yet.")
		ui.renderSnapshot()
		return
	}

	sort.Strings(ids)
	current := ui.activeSessionID()

	var builder strings.Builder
	builder.WriteString("Sessions:")
	count := 0
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		builder.WriteString("\n • ")
		builder.WriteString(id)
		if id == current {
			builder.WriteString(" (current)")
		}
		count++
	}

	if count == 0 {
		ui.appendSystemMessage("No saved sessions yet.")
	} else {
		ui.appendSystemMessage(builder.String())
	}
	ui.renderSnapshot()
}

func (ui *ChatUI) loadSessionCommand(rawID string) {
	id := strings.TrimSpace(rawID)
	if id == "" {
		ui.appendSystemMessage("Usage: /load <session_id>")
		ui.renderSnapshot()
		return
	}

	if ui.coordinator == nil {
		ui.appendSystemMessage("Coordinator unavailable; cannot load session.")
		ui.renderSnapshot()
		return
	}

	if id == ui.activeSessionID() {
		ui.appendSystemMessage(fmt.Sprintf("Session %s is already active.", id))
		ui.renderSnapshot()
		return
	}

	session, err := ui.coordinator.GetSession(ui.ctx, id)
	if err != nil {
		ui.appendSystemMessage(fmt.Sprintf("Failed to load session %s: %v", id, err))
		ui.renderSnapshot()
		return
	}
	if session == nil {
		ui.appendSystemMessage(fmt.Sprintf("Session %s not found.", id))
		ui.renderSnapshot()
		return
	}

	ui.applySessionSnapshot(session)
	ui.appendSystemMessage(fmt.Sprintf("Loaded session %s (%d messages)", session.ID, len(session.Messages)))
	ui.renderSnapshot()
}

func (ui *ChatUI) setVerbose(enabled bool) {
	ui.verbose = enabled
	if ui.outputCtx != nil {
		ui.outputCtx.Verbose = enabled
	}
}

func (ui *ChatUI) verboseCommand(args []string) {
	var message string
	if len(args) == 0 {
		message = fmt.Sprintf("Verbose mode is %s.", verboseState(ui.verbose))
	} else {
		action := strings.ToLower(strings.TrimSpace(args[0]))
		switch action {
		case "on", "enable", "enabled", "true", "1":
			if ui.verbose {
				message = "Verbose mode is already enabled."
			} else {
				ui.setVerbose(true)
				message = "Verbose mode enabled."
			}
		case "off", "disable", "disabled", "false", "0":
			if !ui.verbose {
				message = "Verbose mode is already disabled."
			} else {
				ui.setVerbose(false)
				message = "Verbose mode disabled."
			}
		case "toggle", "flip":
			ui.setVerbose(!ui.verbose)
			if ui.verbose {
				message = "Verbose mode enabled."
			} else {
				message = "Verbose mode disabled."
			}
		default:
			message = "Usage: /verbose [on|off|toggle]"
		}
	}

	ui.appendSystemMessage(message)
	ui.renderSnapshot()
}

func verboseState(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func (ui *ChatUI) followCommand(args []string) {
	usage := "Usage: /follow <transcript|stream|both> <on|off|toggle>"
	status := func() string {
		return fmt.Sprintf(
			"Follow defaults -> transcript=%s, stream=%s. %s",
			followStateShort(ui.defaultFollowTranscript),
			followStateShort(ui.defaultFollowStream),
			usage,
		)
	}

	if len(args) == 0 {
		ui.appendSystemMessage(status())
		ui.renderSnapshot()
		return
	}

	target := strings.ToLower(strings.TrimSpace(args[0]))
	if target == "status" {
		ui.appendSystemMessage(status())
		ui.renderSnapshot()
		return
	}

	if len(args) < 2 {
		ui.appendSystemMessage(usage)
		ui.renderSnapshot()
		return
	}

	action := strings.ToLower(strings.TrimSpace(args[1]))
	transcript := ui.defaultFollowTranscript
	stream := ui.defaultFollowStream
	var message string
	changed := false

	switch target {
	case "transcript", "chat", "history":
		var ok bool
		transcript, message, ok = followAction("Transcript follow", transcript, action, usage)
		if !ok {
			ui.appendSystemMessage(message)
			ui.renderSnapshot()
			return
		}
		changed = transcript != ui.defaultFollowTranscript
	case "stream", "live":
		var ok bool
		stream, message, ok = followAction("Live stream follow", stream, action, usage)
		if !ok {
			ui.appendSystemMessage(message)
			ui.renderSnapshot()
			return
		}
		changed = stream != ui.defaultFollowStream
	case "both", "all":
		switch action {
		case "on", "enable", "enabled", "true", "1":
			changed = !transcript || !stream
			transcript, stream = true, true
			if changed {
				message = "Follow defaults enabled for transcript and live stream."
			} else {
				message = "Follow defaults are already enabled for both panes."
			}
		case "off", "disable", "disabled", "false", "0":
			changed = transcript || stream
			transcript, stream = false, false
			if changed {
				message = "Follow defaults disabled for transcript and live stream."
			} else {
				message = "Follow defaults are already disabled for both panes."
			}
		case "toggle", "flip":
			transcript = !transcript
			stream = !stream
			changed = true
			message = fmt.Sprintf(
				"Follow defaults toggled -> transcript=%s, stream=%s.",
				followStateShort(transcript),
				followStateShort(stream),
			)
		default:
			ui.appendSystemMessage(usage)
			ui.renderSnapshot()
			return
		}
	default:
		ui.appendSystemMessage(usage)
		ui.renderSnapshot()
		return
	}

	if !changed {
		ui.appendSystemMessage(message)
		ui.renderSnapshot()
		return
	}

	ui.defaultFollowTranscript = transcript
	ui.defaultFollowStream = stream
	ui.restoreFollowDefaults()

	var path string
	if ui.saveFollowPreferences != nil {
		if saved, err := ui.saveFollowPreferences(transcript, stream); err != nil {
			ui.appendSystemMessage(fmt.Sprintf("Follow defaults updated but failed to persist: %v", err))
			ui.renderSnapshot()
			return
		} else {
			path = saved
		}
	}

	summary := fmt.Sprintf(
		"Follow defaults updated -> transcript=%s, stream=%s.",
		followStateShort(transcript),
		followStateShort(stream),
	)
	if path != "" {
		summary = fmt.Sprintf("%s Saved to %s.", summary, path)
	}
	if message != "" && !strings.EqualFold(message, usage) {
		summary = fmt.Sprintf("%s %s", message, summary)
	}

	ui.appendSystemMessage(summary)
	ui.renderSnapshot()
}

func followAction(label string, current bool, action, usage string) (bool, string, bool) {
	switch action {
	case "on", "enable", "enabled", "true", "1":
		if current {
			return current, fmt.Sprintf("%s is already enabled.", label), true
		}
		return true, fmt.Sprintf("%s enabled.", label), true
	case "off", "disable", "disabled", "false", "0":
		if !current {
			return current, fmt.Sprintf("%s is already disabled.", label), true
		}
		return false, fmt.Sprintf("%s disabled.", label), true
	case "toggle", "flip":
		next := !current
		return next, fmt.Sprintf("%s %s.", label, followStateLong(next)), true
	default:
		return current, usage, false
	}
}

func followStateShort(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func followStateLong(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func (ui *ChatUI) costCommand(args []string) {
	if ui.costTracker == nil {
		ui.appendSystemMessage("Cost tracker unavailable.")
		ui.renderSnapshot()
		return
	}

	sessionID := strings.TrimSpace(ui.activeSessionID())
	if len(args) > 0 {
		sessionID = strings.TrimSpace(strings.Join(args, " "))
	}

	if sessionID == "" {
		ui.appendSystemMessage("No active session yet. Run a task first or provide an ID: /cost <session_id>.")
		ui.renderSnapshot()
		return
	}

	summary, err := ui.costTracker.GetSessionCost(ui.ctx, sessionID)
	if err != nil {
		ui.appendSystemMessage(fmt.Sprintf("Failed to load cost for session %s: %v", sessionID, err))
		ui.renderSnapshot()
		return
	}

	if summary == nil || isEmptyCostSummary(summary) {
		ui.appendSystemMessage(fmt.Sprintf("No cost data recorded for session %s yet.", sessionID))
		ui.renderSnapshot()
		return
	}

	ui.appendSystemMessage(formatCostSummary(sessionID, summary))
	ui.applyCostSummaryMetrics(summary)
	ui.renderSnapshot()
}

func (ui *ChatUI) mcpCommand(args []string) {
	if ui.registry == nil {
		ui.appendSystemMessage("MCP registry unavailable.")
		ui.renderSnapshot()
		return
	}

	if len(args) == 0 {
		ui.appendSystemMessage(formatMCPServerList(ui.registry.ListServers()))
		ui.renderSnapshot()
		return
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "list":
		ui.appendSystemMessage(formatMCPServerList(ui.registry.ListServers()))
	case "status", "refresh":
		ui.publishMCPStatuses()
		ui.appendSystemMessage("MCP statuses refreshed.")
	case "restart":
		if len(args) < 2 {
			ui.appendSystemMessage("Usage: /mcp restart <name>")
			break
		}
		name := strings.TrimSpace(args[1])
		if name == "" {
			ui.appendSystemMessage("Usage: /mcp restart <name>")
			break
		}
		if ui.hub != nil {
			status := state.MCPStatusStarting
			ui.hub.PublishMCPDelta(state.MCPServerDelta{
				Name:      name,
				Status:    &status,
				Timestamp: time.Now(),
			})
		}
		if err := ui.registry.RestartServer(name); err != nil {
			ui.appendSystemMessage(fmt.Sprintf("Failed to restart MCP server %s: %v", name, err))
			break
		}
		ui.publishMCPStatuses()
		ui.appendSystemMessage(fmt.Sprintf("Restarted MCP server %s", name))
	default:
		ui.appendSystemMessage(fmt.Sprintf("Unknown MCP command: %s", args[0]))
	}

	ui.renderSnapshot()
}

func (ui *ChatUI) exportTranscriptCommand(rawTarget string) {
	snapshot := ui.store.Snapshot()
	if len(snapshot.Messages) == 0 {
		ui.appendSystemMessage("No messages to export yet.")
		ui.renderSnapshot()
		return
	}

	exportedAt := time.Now()
	sessionID := ui.activeSessionID()
	sanitizedSession := sanitizeFilename(sessionID)
	if sanitizedSession == "" {
		sanitizedSession = "session"
	}
	defaultName := fmt.Sprintf("%s-transcript-%s.md", sanitizedSession, exportedAt.Format("20060102-150405"))

	cleaned := strings.TrimSpace(rawTarget)
	target := ""
	if cleaned == "" {
		target = defaultName
	} else {
		wantsDir := strings.HasSuffix(cleaned, string(os.PathSeparator)) || strings.HasSuffix(cleaned, "/") || strings.HasSuffix(cleaned, "\\")
		candidate := filepath.Clean(cleaned)
		if wantsDir {
			target = filepath.Join(candidate, defaultName)
		} else {
			info, err := os.Stat(candidate)
			if err == nil && info.IsDir() {
				target = filepath.Join(candidate, defaultName)
			} else {
				target = candidate
			}
		}
	}

	if dir := filepath.Dir(target); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			ui.appendSystemMessage(fmt.Sprintf("Failed to create export directory: %v", err))
			ui.renderSnapshot()
			return
		}
	}

	content := formatTranscriptExport(snapshot.Messages, sessionID, exportedAt)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		ui.appendSystemMessage(fmt.Sprintf("Failed to export transcript: %v", err))
		ui.renderSnapshot()
		return
	}

	absolute := target
	if abs, err := filepath.Abs(target); err == nil {
		absolute = abs
	}

	ui.appendSystemMessage(fmt.Sprintf("Exported transcript to %s", absolute))
	ui.renderSnapshot()
}

func (ui *ChatUI) executeTask(task string) {
	ui.pending.Add(1)
	ui.renderSnapshot()

	sessionID, err := ui.ensureSession()
	if err != nil {
		ui.store.Apply(state.MessageAppend{Message: state.ChatMessage{
			Role:      state.RoleSystem,
			AgentID:   "system",
			Content:   fmt.Sprintf("Failed to create session: %v", err),
			CreatedAt: time.Now(),
		}})
		ui.pending.Add(-1)
		ui.renderSnapshot()
		return
	}

	listener := eventhub.NewListener(ui.hub)
	ctx := builtin.WithParentListener(ui.ctx, listener)
	if _, err := ui.coordinator.ExecuteTask(ctx, task, sessionID, listener); err != nil {
		ui.store.Apply(state.MessageAppend{Message: state.ChatMessage{
			Role:      state.RoleSystem,
			AgentID:   "system",
			Content:   fmt.Sprintf("Task failed: %v", err),
			CreatedAt: time.Now(),
		}})
	}

	ui.pending.Add(-1)
	ui.renderSnapshot()
}

func (ui *ChatUI) ensureSession() (string, error) {
	ui.sessionMu.Lock()
	defer ui.sessionMu.Unlock()

	if ui.sessionID != "" {
		return ui.sessionID, nil
	}

	session, err := ui.coordinator.GetSession(ui.ctx, "")
	if err != nil {
		return "", err
	}

	ui.sessionID = session.ID
	return ui.sessionID, nil
}

func (ui *ChatUI) renderSnapshot() {
	if ui.app == nil {
		return
	}
	snapshot := ui.store.Snapshot()
	pending := ui.pending.Load()

	transcript := renderTranscript(snapshot.Messages)
	stream := renderLiveStream(snapshot.ToolRuns)
	toolRuns := renderToolRuns(snapshot.ToolRuns)
	subagents := renderSubagents(snapshot.SubagentRuns)
	spinner := ui.currentSpinnerChar()
	mcp := renderMCPServers(snapshot.MCPServers, spinner)
	status := renderStatus(int(pending), snapshot, ui.activeSessionID(), ui.currentSearchSummary(), ui.verbose)

	ui.app.QueueUpdateDraw(func() {
		ui.transcript.SetText(transcript)
		ui.liveStream.SetText(stream)
		ui.tools.SetText(toolRuns)
		ui.subagents.SetText(subagents)
		ui.mcp.SetText(mcp)
		ui.status.SetText(status)
	})
}

func (ui *ChatUI) observeMCP() {
	if ui.registry == nil || ui.hub == nil {
		return
	}

	// Preload configured servers so the UI can show "starting" immediately.
	if loader := mcp.NewConfigLoader(); loader != nil {
		if config, err := loader.Load(); err == nil {
			for name := range config.GetActiveServers() {
				if name == "" {
					continue
				}
				status := state.MCPStatusStarting
				ui.mcpKnown[name] = status
				ui.hub.PublishMCPDelta(state.MCPServerDelta{
					Name:      name,
					Status:    &status,
					Timestamp: time.Now(),
				})
			}
		}
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ui.ctx.Done():
			ui.publishStoppedMCP()
			return
		case <-ticker.C:
			ui.publishMCPStatuses()
		}
	}
}

func (ui *ChatUI) publishMCPStatuses() {
	if ui.registry == nil || ui.hub == nil {
		return
	}

	instances := ui.registry.ListServers()
	seen := make(map[string]struct{}, len(instances))
	now := time.Now()

	for _, instance := range instances {
		if instance == nil || instance.Name == "" {
			continue
		}

		status := translateMCPStatus(instance.Status)
		delta := state.MCPServerDelta{
			Name:      instance.Name,
			Status:    &status,
			Timestamp: now,
		}
		if !instance.StartedAt.IsZero() {
			startedAt := instance.StartedAt
			delta.StartedAt = &startedAt
		}
		if instance.LastError != nil {
			errText := instance.LastError.Error()
			delta.LastError = &errText
		}

		ui.hub.PublishMCPDelta(delta)
		ui.mcpKnown[instance.Name] = status
		seen[instance.Name] = struct{}{}
	}

	if len(ui.mcpKnown) == 0 {
		return
	}

	now = time.Now()
	for name, last := range ui.mcpKnown {
		if _, ok := seen[name]; ok {
			continue
		}
		if last == state.MCPStatusStarting {
			// Allow time for startup before reporting as stopped.
			continue
		}
		status := state.MCPStatusStopped
		ui.hub.PublishMCPDelta(state.MCPServerDelta{
			Name:      name,
			Status:    &status,
			Timestamp: now,
		})
		ui.mcpKnown[name] = status
	}
}

func (ui *ChatUI) publishStoppedMCP() {
	if len(ui.mcpKnown) == 0 || ui.hub == nil {
		return
	}
	now := time.Now()
	status := state.MCPStatusStopped
	for name := range ui.mcpKnown {
		ui.hub.PublishMCPDelta(state.MCPServerDelta{
			Name:      name,
			Status:    &status,
			Timestamp: now,
		})
		ui.mcpKnown[name] = status
	}
}

func (ui *ChatUI) animateMCP() {
	if len(spinnerFrames) == 0 {
		return
	}
	ticker := time.NewTicker(180 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ui.ctx.Done():
			return
		case <-ticker.C:
			snapshot := ui.store.Snapshot()
			if !hasStartingMCP(snapshot.MCPServers) {
				continue
			}
			spinner := ui.nextSpinnerChar()
			mcp := renderMCPServers(snapshot.MCPServers, spinner)
			status := renderStatus(int(ui.pending.Load()), snapshot, ui.activeSessionID(), ui.currentSearchSummary(), ui.verbose)
			ui.app.QueueUpdateDraw(func() {
				ui.mcp.SetText(mcp)
				ui.status.SetText(status)
			})
		}
	}
}

func (ui *ChatUI) nextSpinnerChar() string {
	frames := len(spinnerFrames)
	if frames == 0 {
		return ""
	}
	frame := ui.spinnerFrame.Add(1)
	idx := int(frame) % frames
	if idx < 0 {
		idx += frames
	}
	return spinnerFrames[idx]
}

func (ui *ChatUI) currentSpinnerChar() string {
	frames := len(spinnerFrames)
	if frames == 0 {
		return ""
	}
	frame := ui.spinnerFrame.Load()
	idx := int(frame) % frames
	if idx < 0 {
		idx += frames
	}
	return spinnerFrames[idx]
}

func hasStartingMCP(servers []*state.MCPServer) bool {
	for _, server := range servers {
		if server != nil && server.Status == state.MCPStatusStarting {
			return true
		}
	}
	return false
}

func renderTranscript(messages []state.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}

	var b strings.Builder
	for _, msg := range messages {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatMessage(msg))
	}
	return b.String()
}

func formatMessage(msg state.ChatMessage) string {
	var role string
	switch msg.Role {
	case state.RoleUser:
		role = "[yellow::b]You[-]"
	case state.RoleAssistant:
		role = "[cyan::b]Alex[-]"
	case state.RoleSystem:
		role = "[red::b]System[-]"
	case state.RoleTool:
		role = "[magenta::b]Tool[-]"
	default:
		role = fmt.Sprintf("[%s]", strings.ToUpper(string(msg.Role)))
	}

	if msg.AgentID != "" && msg.Role != state.RoleUser {
		role = fmt.Sprintf("%s [gray](%s)[-]", role, msg.AgentID)
	}

	content := strings.TrimRight(msg.Content, "\r\n")
	if content == "" {
		content = msg.Content
	}

	return fmt.Sprintf("%s\n%s", role, content)
}

func renderLiveStream(runs []*state.ToolRun) string {
	if len(runs) == 0 {
		return "No active tools."
	}

	var active *state.ToolRun
	for _, run := range runs {
		if run == nil {
			continue
		}
		if run.Status == state.ToolStatusRunning {
			if active == nil || run.UpdatedAt.After(active.UpdatedAt) {
				active = run
			}
		}
	}

	if active == nil {
		// Fall back to the most recent streamed tool so history remains visible briefly.
		for _, run := range runs {
			if run == nil || len(run.Stream) == 0 {
				continue
			}
			if active == nil || run.UpdatedAt.After(active.UpdatedAt) {
				active = run
			}
		}
	}

	if active == nil {
		return "No live stream."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[yellow::b]%s[-]", safeToolName(active.ToolName)))
	if active.AgentID != "" {
		b.WriteString(fmt.Sprintf(" [gray](%s)[-]", active.AgentID))
	}

	stream := strings.Join(active.Stream, "")
	if stream == "" {
		return b.String()
	}

	b.WriteString("\n")
	b.WriteString(truncateText(stream, 4000))
	return b.String()
}

func renderToolRuns(runs []*state.ToolRun) string {
	if len(runs) == 0 {
		return "No tools have run yet."
	}

	ordered := append([]*state.ToolRun(nil), runs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i] == nil {
			return false
		}
		if ordered[j] == nil {
			return true
		}
		ti := mostRecentTime(ordered[i])
		tj := mostRecentTime(ordered[j])
		if ti.Equal(tj) {
			return ordered[i].ID > ordered[j].ID
		}
		return ti.After(tj)
	})

	var b strings.Builder
	for _, run := range ordered {
		if run == nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatToolRun(run))
	}

	if b.Len() == 0 {
		return "No tools have run yet."
	}
	return b.String()
}

func renderSubagents(tasks []*state.SubagentTask) string {
	if len(tasks) == 0 {
		return "No subagents running."
	}

	ordered := append([]*state.SubagentTask(nil), tasks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i] == nil {
			return false
		}
		if ordered[j] == nil {
			return true
		}
		if ordered[i].UpdatedAt.Equal(ordered[j].UpdatedAt) {
			return ordered[i].Index < ordered[j].Index
		}
		return ordered[i].UpdatedAt.After(ordered[j].UpdatedAt)
	})

	var b strings.Builder
	for _, task := range ordered {
		if task == nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatSubagent(task))
	}

	if b.Len() == 0 {
		return "No subagents running."
	}
	return b.String()
}

func renderMCPServers(servers []*state.MCPServer, spinner string) string {
	if len(servers) == 0 {
		return "No MCP servers configured."
	}

	var b strings.Builder
	for _, server := range servers {
		if server == nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatMCPServer(server, spinner))
	}

	if b.Len() == 0 {
		return "No MCP servers configured."
	}
	return b.String()
}

func formatMCPServerList(instances []*mcp.ServerInstance) string {
	filtered := make([]*mcp.ServerInstance, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		name := strings.TrimSpace(instance.Name)
		if name == "" {
			continue
		}
		filtered = append(filtered, instance)
	}

	if len(filtered) == 0 {
		return "No MCP servers configured."
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

	var b strings.Builder
	b.WriteString("MCP servers:")

	for _, instance := range filtered {
		status := translateMCPStatus(instance.Status)
		b.WriteString("\n • ")
		b.WriteString(instance.Name)
		b.WriteString(" – ")
		b.WriteString(strings.ToLower(string(status)))
		if status == state.MCPStatusReady && !instance.StartedAt.IsZero() {
			b.WriteString(" since ")
			b.WriteString(instance.StartedAt.Format("15:04"))
		}
		if instance.RestartCount > 0 {
			b.WriteString(fmt.Sprintf("\n    restarts: %d", instance.RestartCount))
		}
		if instance.LastError != nil {
			text := strings.TrimSpace(instance.LastError.Error())
			if text != "" {
				b.WriteString("\n    last error: ")
				b.WriteString(truncateText(text, 200))
			}
		}
	}

	return b.String()
}

func renderStatus(pending int, snapshot state.Snapshot, sessionID, searchSummary string, verbose bool) string {
	var sections []string

	switch {
	case pending <= 0:
		sections = append(sections, "[green::b]Ready[-]")
	case pending == 1:
		sections = append(sections, "[yellow::b]Running task…[-]")
	default:
		sections = append(sections, fmt.Sprintf("[yellow::b]Running %d tasks…[-]", pending))
	}

	trimmedSession := strings.TrimSpace(sessionID)
	if trimmedSession == "" {
		sections = append(sections, "[white]Session: (new)[-]")
	} else {
		sections = append(sections, fmt.Sprintf("[white]Session: %s[-]", trimmedSession))
	}

	runningTools := 0
	for _, run := range snapshot.ToolRuns {
		if run != nil && run.Status == state.ToolStatusRunning {
			runningTools++
		}
	}
	sections = append(sections, fmt.Sprintf("[cyan]Tools: %d active[-]", runningTools))

	readyMCP := 0
	for _, server := range snapshot.MCPServers {
		if server != nil && server.Status == state.MCPStatusReady {
			readyMCP++
		}
	}
	if len(snapshot.MCPServers) > 0 {
		sections = append(sections, fmt.Sprintf("[magenta]MCP: %d/%d ready[-]", readyMCP, len(snapshot.MCPServers)))
	} else {
		sections = append(sections, "[magenta]MCP: none[-]")
	}

	activeSubagents := 0
	for _, task := range snapshot.SubagentRuns {
		if task != nil && task.Status == state.SubtaskStatusRunning {
			activeSubagents++
		}
	}
	sections = append(sections, fmt.Sprintf("[blue]Subagents: %d running[-]", activeSubagents))

	if tokenSection := renderTokenSummary(snapshot.Metrics); tokenSection != "" {
		sections = append(sections, tokenSection)
	}
	if costSection := renderCostSummary(snapshot.Metrics); costSection != "" {
		sections = append(sections, costSection)
	}

	if strings.TrimSpace(searchSummary) != "" {
		sections = append(sections, fmt.Sprintf("[white]%s[-]", searchSummary))
	}

	if verbose {
		sections = append(sections, "[gray]Verbose: on[-]")
	} else {
		sections = append(sections, "[gray]Verbose: off[-]")
	}

	sections = append(sections, "[gray]? Help  Tab focus  End follow  / search  n/N next[-]")

	return strings.Join(sections, "  •  ")
}

func renderTokenSummary(metrics state.Metrics) string {
	if metrics.TotalTokens <= 0 {
		return ""
	}

	var keys []string
	for agent, tokens := range metrics.TokensByAgent {
		if tokens <= 0 {
			continue
		}
		keys = append(keys, agent)
	}
	sort.Strings(keys)

	var parts []string
	for _, agent := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", agent, metrics.TokensByAgent[agent]))
	}

	if len(parts) == 0 {
		return fmt.Sprintf("[white]Tokens: %d[-]", metrics.TotalTokens)
	}

	return fmt.Sprintf("[white]Tokens: %d (%s)[-]", metrics.TotalTokens, strings.Join(parts, ", "))
}

func renderCostSummary(metrics state.Metrics) string {
	if metrics.TotalCost <= 0 {
		return ""
	}

	summary := fmt.Sprintf("[white]Cost: $%.4f", metrics.TotalCost)

	var keys []string
	for model, cost := range metrics.CostByModel {
		if cost <= 0 {
			continue
		}
		keys = append(keys, model)
	}
	sort.Strings(keys)

	if len(keys) > 0 {
		parts := make([]string, 0, len(keys))
		for _, model := range keys {
			parts = append(parts, fmt.Sprintf("%s=$%.4f", model, metrics.CostByModel[model]))
		}
		summary = fmt.Sprintf("%s (%s)", summary, strings.Join(parts, ", "))
	}

	return summary + "[-]"
}

func isEmptyCostSummary(summary *ports.CostSummary) bool {
	if summary == nil {
		return true
	}

	if summary.TotalCost != 0 || summary.RequestCount != 0 || summary.TotalTokens != 0 || summary.InputTokens != 0 || summary.OutputTokens != 0 {
		return false
	}

	for _, cost := range summary.ByModel {
		if cost > 0 {
			return false
		}
	}
	for _, cost := range summary.ByProvider {
		if cost > 0 {
			return false
		}
	}

	return true
}

func formatCostSummary(sessionID string, summary *ports.CostSummary) string {
	if summary == nil {
		return fmt.Sprintf("No cost data recorded for session %s yet.", sessionID)
	}

	trimmedSession := strings.TrimSpace(sessionID)
	if trimmedSession == "" {
		trimmedSession = "(unsaved)"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Cost for session %s:", trimmedSession))
	b.WriteString(fmt.Sprintf("\n • Total: $%.4f", summary.TotalCost))
	b.WriteString(fmt.Sprintf("\n • Requests: %d", summary.RequestCount))

	if summary.InputTokens > 0 || summary.OutputTokens > 0 || summary.TotalTokens > 0 {
		b.WriteString(fmt.Sprintf("\n • Tokens: input=%d output=%d total=%d", summary.InputTokens, summary.OutputTokens, summary.TotalTokens))
	}

	if len(summary.ByModel) > 0 {
		var models []string
		for model, cost := range summary.ByModel {
			if cost <= 0 {
				continue
			}
			models = append(models, model)
		}
		sort.Strings(models)
		if len(models) > 0 {
			parts := make([]string, 0, len(models))
			for _, model := range models {
				parts = append(parts, fmt.Sprintf("%s=$%.4f", model, summary.ByModel[model]))
			}
			b.WriteString("\n • Models: ")
			b.WriteString(strings.Join(parts, ", "))
		}
	}

	if len(summary.ByProvider) > 0 {
		var providers []string
		for provider, cost := range summary.ByProvider {
			if cost <= 0 {
				continue
			}
			providers = append(providers, provider)
		}
		sort.Strings(providers)
		if len(providers) > 0 {
			parts := make([]string, 0, len(providers))
			for _, provider := range providers {
				parts = append(parts, fmt.Sprintf("%s=$%.4f", provider, summary.ByProvider[provider]))
			}
			b.WriteString("\n • Providers: ")
			b.WriteString(strings.Join(parts, ", "))
		}
	}

	if !summary.StartTime.IsZero() || !summary.EndTime.IsZero() {
		start := summary.StartTime
		end := summary.EndTime
		if start.IsZero() {
			start = end
		}
		if end.IsZero() {
			end = time.Now()
		}
		b.WriteString("\n • Window: ")
		if !start.IsZero() {
			b.WriteString(start.UTC().Format(time.RFC3339))
		} else {
			b.WriteString("unknown")
		}
		b.WriteString(" → ")
		if !summary.EndTime.IsZero() {
			b.WriteString(end.UTC().Format(time.RFC3339))
		} else {
			b.WriteString("ongoing")
		}
	}

	return b.String()
}

func formatToolRun(run *state.ToolRun) string {
	if run == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(toolStatusIndicator(run.Status))
	b.WriteString(" ")
	b.WriteString(fmt.Sprintf("[::b]%s[-]", safeToolName(run.ToolName)))
	if run.AgentID != "" {
		b.WriteString(fmt.Sprintf(" [gray](%s)[-]", run.AgentID))
	}

	if run.Status == state.ToolStatusRunning && run.StartedAt != nil {
		b.WriteString(fmt.Sprintf(" [gray]since %s[-]", run.StartedAt.Format("15:04:05")))
	}
	if run.Status == state.ToolStatusCompleted && run.Duration > 0 {
		b.WriteString(fmt.Sprintf(" [gray]%.1fs[-]", run.Duration.Seconds()))
	}

	if run.Result != "" {
		b.WriteString("\n[gray]")
		b.WriteString(truncateText(run.Result, 600))
		b.WriteString("[-]")
	}

	if run.Error != "" {
		b.WriteString("\n[red::b]Error:[-] ")
		b.WriteString(truncateText(run.Error, 400))
	}

	return b.String()
}

func formatSubagent(task *state.SubagentTask) string {
	indicator := subagentStatusIndicator(task.Status)
	preview := task.Preview
	if preview == "" {
		preview = "Subtask"
	}

	var b strings.Builder
	b.WriteString(indicator)
	b.WriteString(" ")
	b.WriteString(fmt.Sprintf("[::b]%s[-]", truncateText(preview, 120)))
	if task.AgentLevel != "" {
		b.WriteString(fmt.Sprintf(" [gray](%s)[-]", task.AgentLevel))
	}
	if task.Total > 0 {
		b.WriteString(fmt.Sprintf(" [gray](%d/%d tools)[-]", task.ToolsCompleted, task.Total))
	}
	switch task.Status {
	case state.SubtaskStatusRunning:
		if task.StartedAt != nil {
			b.WriteString(fmt.Sprintf(" [gray]since %s[-]", task.StartedAt.Format("15:04:05")))
		}
	case state.SubtaskStatusCompleted, state.SubtaskStatusFailed:
		if task.Duration > 0 {
			b.WriteString(fmt.Sprintf(" [gray]%.1fs[-]", task.Duration.Seconds()))
		}
		if task.CompletedAt != nil {
			b.WriteString(fmt.Sprintf(" [gray]finished %s[-]", task.CompletedAt.Format("15:04:05")))
		}
	}
	if task.CurrentTool != "" {
		b.WriteString(fmt.Sprintf("\n[gray]Running:[-] %s", truncateText(task.CurrentTool, 80)))
	}
	if task.TokensUsed > 0 {
		b.WriteString(fmt.Sprintf("\n[gray]Tokens:[-] %d", task.TokensUsed))
	}
	if task.Error != "" {
		b.WriteString("\n[red::b]Error:[-] ")
		b.WriteString(truncateText(task.Error, 400))
	}
	return b.String()
}

func formatMCPServer(server *state.MCPServer, spinner string) string {
	indicator := mcpStatusIndicator(server.Status, spinner)
	var b strings.Builder
	b.WriteString(indicator)
	b.WriteString(" ")
	b.WriteString(fmt.Sprintf("[::b]%s[-]", server.Name))

	switch server.Status {
	case state.MCPStatusReady:
		if server.StartedAt != nil {
			b.WriteString(fmt.Sprintf(" [gray]since %s[-]", server.StartedAt.Format("15:04")))
		}
	case state.MCPStatusError:
		if server.LastError != "" {
			b.WriteString("\n[red::b]Error:[-] ")
			b.WriteString(truncateText(server.LastError, 400))
		}
	case state.MCPStatusStarting:
		b.WriteString(" [gray]starting…[-]")
	case state.MCPStatusStopped:
		b.WriteString(" [gray]stopped[-]")
	}

	return b.String()
}

func mostRecentTime(run *state.ToolRun) time.Time {
	if run == nil {
		return time.Time{}
	}
	if run.CompletedAt != nil {
		return *run.CompletedAt
	}
	if !run.UpdatedAt.IsZero() {
		return run.UpdatedAt
	}
	if run.StartedAt != nil {
		return *run.StartedAt
	}
	return time.Time{}
}

func safeToolName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "tool"
	}
	return name
}

func toolStatusIndicator(status state.ToolStatus) string {
	switch status {
	case state.ToolStatusRunning:
		return "[yellow]⏳[-]"
	case state.ToolStatusCompleted:
		return "[green]✔[-]"
	case state.ToolStatusFailed:
		return "[red]✖[-]"
	default:
		return "[gray]•[-]"
	}
}

func subagentStatusIndicator(status state.SubtaskStatus) string {
	switch status {
	case state.SubtaskStatusRunning:
		return "[yellow]▶[-]"
	case state.SubtaskStatusCompleted:
		return "[green]✔[-]"
	case state.SubtaskStatusFailed:
		return "[red]✖[-]"
	default:
		return "[gray]○[-]"
	}
}

func mcpStatusIndicator(status state.MCPStatus, spinner string) string {
	switch status {
	case state.MCPStatusReady:
		return "[green]●[-]"
	case state.MCPStatusStarting:
		if strings.TrimSpace(spinner) != "" {
			return fmt.Sprintf("[yellow]%s[-]", spinner)
		}
		return "[yellow]●[-]"
	case state.MCPStatusError:
		return "[red]●[-]"
	case state.MCPStatusStopped:
		return "[gray]●[-]"
	default:
		return "[gray]○[-]"
	}
}

func truncateText(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}

func formatTranscriptExport(messages []state.ChatMessage, sessionID string, exportedAt time.Time) string {
	var b strings.Builder
	b.WriteString("# Alex Chat Transcript\n\n")
	b.WriteString(fmt.Sprintf("- Exported: %s\n", exportedAt.UTC().Format(time.RFC3339)))
	if strings.TrimSpace(sessionID) != "" {
		b.WriteString(fmt.Sprintf("- Session: %s\n", sessionID))
	} else {
		b.WriteString("- Session: (unsaved)\n")
	}
	b.WriteString(fmt.Sprintf("- Messages: %d\n\n", len(messages)))

	for _, msg := range messages {
		timestamp := msg.CreatedAt
		if timestamp.IsZero() {
			timestamp = exportedAt
		}
		role := formatExportRole(msg.Role)
		b.WriteString(fmt.Sprintf("## [%s] %s", timestamp.UTC().Format(time.RFC3339), role))
		if agent := strings.TrimSpace(msg.AgentID); agent != "" {
			b.WriteString(fmt.Sprintf(" (%s)", agent))
		}
		b.WriteString("\n")
		body := indentMultiline(strings.TrimRight(msg.Content, "\n"))
		if body != "" {
			b.WriteString(body)
		}
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func indentMultiline(text string) string {
	if text == "" {
		return "    (empty)"
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

func formatExportRole(role state.ChatRole) string {
	switch role {
	case state.RoleUser:
		return "User"
	case state.RoleAssistant:
		return "Assistant"
	case state.RoleTool:
		return "Tool"
	case state.RoleSystem:
		return "System"
	default:
		value := strings.TrimSpace(string(role))
		if value == "" {
			return "Unknown"
		}
		return capitalize(value)
	}
}

func capitalize(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

func sanitizeFilename(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		switch r {
		case '-', '_':
			return r
		default:
			return '_'
		}
	}, trimmed)

	cleaned = strings.Trim(cleaned, "_.-")
	cleaned = strings.ReplaceAll(cleaned, "__", "_")
	cleaned = strings.Trim(cleaned, "_.-")
	return cleaned
}

func translateMCPStatus(status mcp.ServerStatus) state.MCPStatus {
	switch status {
	case mcp.StatusRunning:
		return state.MCPStatusReady
	case mcp.StatusStarting:
		return state.MCPStatusStarting
	case mcp.StatusError:
		return state.MCPStatusError
	case mcp.StatusStopped:
		return state.MCPStatusStopped
	default:
		return state.MCPStatusUnknown
	}
}

func convertSessionMessage(msg ports.Message) []state.ChatMessage {
	var messages []state.ChatMessage
	agentID := metadataAgentID(msg.Metadata)
	role := mapSessionRole(msg.Role)

	if strings.TrimSpace(msg.Content) != "" {
		messages = append(messages, state.ChatMessage{
			Role:    role,
			AgentID: agentID,
			Content: msg.Content,
		})
	}

	for _, result := range msg.ToolResults {
		id := agentID
		if metaID := metadataAgentID(result.Metadata); metaID != "" {
			id = metaID
		}
		if result.Error != nil {
			messages = append(messages, state.ChatMessage{
				Role:    state.RoleSystem,
				AgentID: id,
				Content: fmt.Sprintf("Tool error: %v", result.Error),
			})
			continue
		}
		if strings.TrimSpace(result.Content) == "" {
			continue
		}
		messages = append(messages, state.ChatMessage{
			Role:    state.RoleTool,
			AgentID: id,
			Content: result.Content,
		})
	}

	return messages
}

func metadataAgentID(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata["agent_id"]; ok {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return ""
}

func mapSessionRole(role string) state.ChatRole {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system":
		return state.RoleSystem
	case "assistant":
		return state.RoleAssistant
	case "tool":
		return state.RoleTool
	case "user":
		return state.RoleUser
	default:
		if role == "" {
			return state.RoleAssistant
		}
		return state.ChatRole(role)
	}
}

func findMatchingLines(text, query string) []int {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil
	}

	normalized := normalizeNewlines(text)
	lowerQuery := strings.ToLower(trimmed)
	lines := strings.Split(normalized, "\n")
	matches := make([]int, 0)
	for idx, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerQuery) {
			matches = append(matches, idx)
		}
	}
	return matches
}

func normalizeNewlines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return value
}
