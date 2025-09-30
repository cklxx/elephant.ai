//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unsafe"
)

// CursorPosition represents a cursor position
type CursorPosition struct {
	X, Y int
}

// TerminalController handles fixed bottom interface with proper state management
type TerminalController struct {
	mu              sync.RWMutex     // Protect concurrent access
	height          int              // Terminal height
	width           int              // Terminal width
	bottomLines     int              // Lines reserved for bottom interface
	scrollRegionTop int              // Top line of scroll region
	scrollRegionBot int              // Bottom line of scroll region
	initialized     bool             // Whether terminal is initialized
	cursorStack     []CursorPosition // Stack for cursor positions
	useAltScreen    bool             // Whether to use alternate screen buffer
	currentCursor   CursorPosition   // Track current cursor position
	inputLine       int              // Current input line position
	inputBuffer     string           // Current input buffer
	rawMode         bool             // Whether terminal is in raw mode
	originalState   *syscall.Termios // Original terminal state for restoration
	contentHeight   int              // Current content height
	scrollOffset    int              // Current scroll offset
}

// NewTerminalController creates a new terminal controller
func NewTerminalController() *TerminalController {
	tc := &TerminalController{
		bottomLines:   5, // Reserve 5 lines for working indicator + input box + spacing
		cursorStack:   make([]CursorPosition, 0, 10),
		useAltScreen:  false, // Disable alternate screen for stability
		currentCursor: CursorPosition{X: 1, Y: 1},
		contentHeight: 0, // Initialize content height
		scrollOffset:  0, // Initialize scroll offset
	}
	tc.detectTerminalCapabilities()
	tc.getTerminalSize()
	tc.calculateScrollRegion()
	return tc
}

// detectTerminalCapabilities checks what terminal features are available
func (tc *TerminalController) detectTerminalCapabilities() {
	// For now, disable alternate screen to avoid conflicts
	tc.useAltScreen = false
}

// getTerminalSize gets the current terminal size
func (tc *TerminalController) getTerminalSize() {
	// Try to get terminal size using stty command first
	if width, height := tc.getTerminalSizeANSI(); width > 0 && height > 0 {
		tc.width = width
		tc.height = height
		return
	}

	// Fallback to system call
	if width, height := tc.getTerminalSizeIoctl(); width > 0 && height > 0 {
		tc.width = width
		tc.height = height
		return
	}

	// Default fallback
	tc.width = 80
	tc.height = 24
}

// getTerminalSizeANSI gets terminal size using stty command
func (tc *TerminalController) getTerminalSizeANSI() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(string(output))
	if len(parts) != 2 {
		return 0, 0
	}

	height, err1 := strconv.Atoi(parts[0])
	width, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0
	}

	return width, height
}

// getTerminalSizeIoctl gets terminal size using system call
func (tc *TerminalController) getTerminalSizeIoctl() (int, int) {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}

	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)))

	if int(retCode) == -1 {
		_ = errno
		return 0, 0
	}
	return int(ws.Col), int(ws.Row)
}

// calculateScrollRegion calculates the scroll region boundaries
func (tc *TerminalController) calculateScrollRegion() {
	tc.scrollRegionTop = 1
	tc.scrollRegionBot = tc.height - tc.bottomLines
	if tc.scrollRegionBot < tc.scrollRegionTop {
		tc.scrollRegionBot = tc.scrollRegionTop
	}
	tc.inputLine = tc.height - 2 // Input box content line
}

// pushCursorPosition saves current cursor position to stack
func (tc *TerminalController) pushCursorPosition(x, y int) {
	tc.cursorStack = append(tc.cursorStack, CursorPosition{X: x, Y: y})
}

// moveCursor moves cursor to specific position and updates tracking
func (tc *TerminalController) moveCursor(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
	tc.currentCursor = CursorPosition{X: x, Y: y}
}

// Initialize sets up the terminal for fixed bottom interface
func (tc *TerminalController) Initialize() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.initialized {
		return
	}

	// Clear screen and move to home
	fmt.Print("\033[2J")
	tc.moveCursor(1, 1)

	// Don't use scroll regions - they cause conflicts
	// Instead, we'll manage positioning manually

	// Save initial cursor position in our stack
	tc.pushCursorPosition(1, tc.scrollRegionTop)
	tc.moveCursor(1, tc.scrollRegionTop)

	tc.initialized = true
}

// Cleanup restores normal terminal behavior
func (tc *TerminalController) Cleanup() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.initialized {
		return
	}

	// Disable raw mode first
	if err := tc.disableRawMode(); err != nil {
		// Continue cleanup even if disableRawMode fails - intentionally empty
		_ = err // Suppress staticcheck warning
	}

	// Move cursor to bottom of scroll region instead of absolute bottom
	tc.moveCursor(1, tc.scrollRegionBot+1)
	fmt.Print("\n")

	tc.initialized = false
}

// calculateDynamicPositions calculates working indicator and input box positions
func (tc *TerminalController) calculateDynamicPositions() (int, int) {
	availableHeight := tc.scrollRegionBot - tc.scrollRegionTop + 1

	var workingLine, inputStartLine int

	if tc.contentHeight <= availableHeight-4 {
		// Content fits with room for input box
		workingLine = tc.scrollRegionTop + tc.contentHeight + 1
		inputStartLine = workingLine + 2
	} else {
		// Content exceeds available space, position at bottom
		workingLine = tc.height - 4
		inputStartLine = tc.height - 2
	}

	// Ensure we don't go beyond terminal bounds
	if inputStartLine+2 >= tc.height {
		inputStartLine = tc.height - 3
		workingLine = inputStartLine - 2
	}

	return workingLine, inputStartLine
}

// ShowDynamicBottomInterface displays the bottom interface following content
func (tc *TerminalController) ShowDynamicBottomInterface(workingIndicator, inputBox string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.initialized {
		return
	}

	// Calculate dynamic positions
	workingLine, inputStartLine := tc.calculateDynamicPositions()

	// Clear and show working indicator
	tc.moveCursor(1, workingLine)
	fmt.Print("\033[2K") // Clear entire line
	if workingIndicator != "" {
		fmt.Print(workingIndicator)
	}

	// Clear and show input box
	tc.moveCursor(1, inputStartLine)
	fmt.Print("\033[2K") // Clear line
	tc.moveCursor(1, inputStartLine+1)
	fmt.Print("\033[2K") // Clear line
	tc.moveCursor(1, inputStartLine+2)
	fmt.Print("\033[2K") // Clear line

	// Show input box
	if inputBox != "" {
		tc.moveCursor(1, inputStartLine)
		fmt.Print(inputBox)
	}

	// Update input line position
	tc.inputLine = inputStartLine + 1

	// Position cursor at leftmost position of input box content area
	leftCol := 3                             // 3 for "│ " (left border + space)
	tc.moveCursor(leftCol, inputStartLine+1) // Content line of input box, left-aligned
}

// ShowBottomInterface displays the bottom interface with dynamic positioning
func (tc *TerminalController) ShowBottomInterface(workingIndicator, inputBox string) {
	tc.ShowDynamicBottomInterface(workingIndicator, inputBox)
}

// ShowFixedBottomInterface displays the fixed bottom interface (backward compatibility)
func (tc *TerminalController) ShowFixedBottomInterface(workingIndicator, inputBox string) {
	// Use dynamic interface for better UX
	tc.ShowDynamicBottomInterface(workingIndicator, inputBox)
}

// PrintInScrollRegion prints content in the scroll region with smart scrolling
func (tc *TerminalController) PrintInScrollRegion(content string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.initialized {
		fmt.Print(content)
		return
	}

	// Save current position
	currentX, currentY := tc.currentCursor.X, tc.currentCursor.Y

	// Calculate content lines
	contentLines := strings.Count(content, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		contentLines++ // Count the last line even if no newline
	}

	// Update content height
	tc.contentHeight += contentLines

	// Calculate available scroll region height
	availableHeight := tc.scrollRegionBot - tc.scrollRegionTop + 1

	// Auto-scroll logic: only scroll if content exceeds available space
	if tc.contentHeight > availableHeight {
		// Calculate how much we need to scroll
		scrollNeeded := tc.contentHeight - availableHeight

		// If we need to scroll, print at current position and let terminal scroll naturally
		fmt.Print(content)

		// Update scroll offset
		tc.scrollOffset = scrollNeeded
	} else {
		// Content fits in available space, print normally
		fmt.Print(content)
	}

	// Update cursor tracking
	if contentLines > 0 {
		if strings.HasSuffix(content, "\n") {
			tc.currentCursor.Y += contentLines
			tc.currentCursor.X = 1
		} else {
			tc.currentCursor.Y += contentLines - 1
			lastLine := content[strings.LastIndex(content, "\n")+1:]
			tc.currentCursor.X = len(lastLine) + 1
		}
	} else {
		tc.currentCursor.X += len(content)
	}

	// Keep cursor within bounds
	if tc.currentCursor.Y > tc.scrollRegionBot {
		tc.currentCursor.Y = tc.scrollRegionBot
	}

	// Restore cursor to input area if it was there
	if currentY >= tc.inputLine {
		tc.moveCursor(currentX, currentY)
	}
}

// UpdateWorkingIndicator updates just the working indicator line using dynamic position
func (tc *TerminalController) UpdateWorkingIndicator(indicator string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.initialized {
		return
	}

	// Save current position
	currentX, currentY := tc.currentCursor.X, tc.currentCursor.Y

	// Calculate dynamic working indicator position
	workingLine, _ := tc.calculateDynamicPositions()

	// Update working indicator
	tc.moveCursor(1, workingLine)
	fmt.Print("\033[2K") // Clear line
	if indicator != "" {
		fmt.Print(indicator)
	}

	// Restore cursor position
	tc.moveCursor(currentX, currentY)
}

// disableRawMode restores original terminal mode
func (tc *TerminalController) disableRawMode() error {
	if !tc.rawMode || tc.originalState == nil {
		return nil
	}

	// For macOS/Darwin, use specific constant
	const TCSETS = 0x80487414

	// Restore original terminal state
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(TCSETS),
		uintptr(unsafe.Pointer(tc.originalState)))

	if errno != 0 {
		return errno
	}

	tc.rawMode = false
	return nil
}

// ReadInputNonBlocking reads input without blocking
func (tc *TerminalController) ReadInputNonBlocking() ([]byte, error) {
	if !tc.rawMode {
		return nil, fmt.Errorf("terminal not in raw mode")
	}

	buffer := make([]byte, 1024)
	n, err := syscall.Read(syscall.Stdin, buffer)
	if err != nil {
		return nil, err
	}

	if n == 0 {
		return nil, nil // No input available
	}

	return buffer[:n], nil
}

// ProcessInputBuffer processes keyboard input and updates the input buffer with UTF-8 support
func (tc *TerminalController) ProcessInputBuffer(input []byte) (string, bool) {
	// Process bytes and convert to UTF-8 string for proper Unicode handling
	inputStr := string(input)

	for _, r := range inputStr {
		switch r {
		case 13, 10: // Enter (CR or LF)
			result := tc.inputBuffer
			tc.inputBuffer = ""
			return result, true
		case 127, 8: // Backspace/Delete
			if len(tc.inputBuffer) > 0 {
				// Handle UTF-8 properly - remove last rune, not last byte
				runes := []rune(tc.inputBuffer)
				if len(runes) > 0 {
					tc.inputBuffer = string(runes[:len(runes)-1])
				}
			}
		case 3: // Ctrl+C
			return "exit", true
		case 27: // Escape - could be start of escape sequence
			// Handle escape sequences if needed
			continue
		default:
			// Add printable characters, including Unicode characters like Chinese
			if unicode.IsPrint(r) || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSymbol(r) || unicode.IsPunct(r) {
				tc.inputBuffer += string(r)
			}
		}
	}
	return "", false
}

// UpdateInputDisplay updates the display of the current input buffer with Unicode width support
func (tc *TerminalController) UpdateInputDisplay() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.initialized {
		return
	}

	// Save current position
	currentX, currentY := tc.currentCursor.X, tc.currentCursor.Y

	// Calculate dynamic input line position
	_, inputStartLine := tc.calculateDynamicPositions()
	tc.moveCursor(3, inputStartLine+1) // Content line of input box

	// Clear the input area and show current buffer
	maxInputWidth := tc.width - 6 // Account for borders and padding
	displayText := tc.inputBuffer

	// Handle Unicode width properly for Chinese characters
	displayWidth := tc.calculateDisplayWidth(displayText)
	if displayWidth > maxInputWidth {
		// Truncate from the left to fit in available space
		runes := []rune(displayText)
		for len(runes) > 0 {
			if tc.calculateDisplayWidth(string(runes)) <= maxInputWidth {
				break
			}
			runes = runes[1:]
		}
		displayText = string(runes)
	}

	fmt.Printf("\033[2K")                      // Clear line from cursor
	fmt.Printf("\033[%d;3H", inputStartLine+1) // Move to input position
	fmt.Print(displayText)

	// Position cursor at the end of displayed text
	cursorX := 3 + tc.calculateDisplayWidth(displayText)
	tc.moveCursor(cursorX, inputStartLine+1)

	// Update tracked input line position
	tc.inputLine = inputStartLine + 1

	// Restore cursor position if it wasn't in input area
	if currentY < tc.inputLine {
		tc.moveCursor(currentX, currentY)
	}
}

// calculateDisplayWidth calculates the display width of a string, accounting for wide characters
func (tc *TerminalController) calculateDisplayWidth(s string) int {
	width := 0
	for _, r := range s {
		if r < 127 {
			// ASCII characters
			width++
		} else if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			// East Asian wide characters (Chinese, Japanese, Korean)
			width += 2
		} else {
			// Other Unicode characters, assume width 1
			width++
		}
	}
	return width
}
