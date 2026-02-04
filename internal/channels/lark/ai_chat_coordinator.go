package lark

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"alex/internal/logging"
)

// AIChatCoordinator manages multi-bot chat sessions to prevent infinite loops
// and coordinate turn-taking when multiple bots are mentioned in a group chat.
type AIChatCoordinator struct {
	mu       sync.RWMutex
	sessions map[string]*aiChatSession // chatID -> session
	logger   logging.Logger
	// Bot IDs that should participate in coordinated chats
	botIDs map[string]bool
}

// aiChatSession tracks the state of a multi-bot chat
type aiChatSession struct {
	chatID          string
	participants    []string // ordered list of bot IDs participating
	currentTurn     int      // index of whose turn it is
	lastActivity    time.Time
	userMessageID   string   // original user message that triggered the session
	userSenderID    string   // original user who initiated the chat
	messageCount    int      // number of messages exchanged
	maxMessages     int      // safety limit to prevent infinite loops
	isActive        bool
}

// NewAIChatCoordinator creates a new coordinator for managing multi-bot chats.
func NewAIChatCoordinator(logger logging.Logger, botIDs []string) *AIChatCoordinator {
	botIDMap := make(map[string]bool, len(botIDs))
	for _, id := range botIDs {
		botIDMap[id] = true
	}
	return &AIChatCoordinator{
		sessions: make(map[string]*aiChatSession),
		logger:   logging.OrNop(logger),
		botIDs:   botIDMap,
	}
}

// DetectAndStartSession checks if a message should trigger a multi-bot chat session.
// Returns true if this bot should participate and wait for its turn.
func (c *AIChatCoordinator) DetectAndStartSession(chatID, messageID, senderID string, mentions []string, thisBotID string) (shouldParticipate bool, waitForTurn bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if this is a multi-bot mention scenario
	mentionedBots := c.extractMentionedBots(mentions)
	if len(mentionedBots) < 2 {
		// Not a multi-bot chat, proceed normally
		return false, false
	}

	// Check if this bot is in the mentioned bots list
	thisBotMentioned := false
	for _, bot := range mentionedBots {
		if bot == thisBotID {
			thisBotMentioned = true
			break
		}
	}
	if !thisBotMentioned {
		return false, false
	}

	// Check if session already exists
	session, exists := c.sessions[chatID]
	if exists && session.isActive {
		// Session exists, check if it's our turn
		if session.participants[session.currentTurn] == thisBotID {
			return true, false // It's our turn, should participate now
		}
		return true, true // Should participate but wait
	}

	// Create new session
	participants := c.orderParticipants(mentionedBots, thisBotID)
	session = &aiChatSession{
		chatID:        chatID,
		participants:  participants,
		currentTurn:   0,
		lastActivity:  time.Now(),
		userMessageID: messageID,
		userSenderID:  senderID,
		messageCount:  0,
		maxMessages:   10, // Safety limit
		isActive:      true,
	}
	c.sessions[chatID] = session

	c.logger.Info("AI chat session started: chat=%s participants=%v", chatID, participants)

	// Check if it's our turn (first bot responds first)
	if participants[0] == thisBotID {
		return true, false
	}
	return true, true
}

// ShouldBotRespond checks if it's this bot's turn to respond in an active session.
func (c *AIChatCoordinator) ShouldBotRespond(chatID, botID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, exists := c.sessions[chatID]
	if !exists || !session.isActive {
		return false
	}

	return session.participants[session.currentTurn] == botID
}

// AdvanceTurn marks the current bot's turn as complete and moves to the next participant.
// Returns the next bot ID that should respond, or empty string if session ended.
func (c *AIChatCoordinator) AdvanceTurn(chatID, botID string) (nextBotID string, shouldContinue bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, exists := c.sessions[chatID]
	if !exists || !session.isActive {
		return "", false
	}

	// Verify it's this bot's turn
	if session.participants[session.currentTurn] != botID {
		return "", false
	}

	session.messageCount++
	session.lastActivity = time.Now()

	// Check if we've reached the message limit
	if session.messageCount >= session.maxMessages {
		c.logger.Info("AI chat session ended: message limit reached for chat=%s", chatID)
		session.isActive = false
		return "", false
	}

	// Move to next turn
	session.currentTurn = (session.currentTurn + 1) % len(session.participants)
	nextBotID = session.participants[session.currentTurn]

	c.logger.Info("AI chat turn advanced: chat=%s next=%s count=%d", chatID, nextBotID, session.messageCount)

	return nextBotID, true
}

// EndSession manually ends a chat session.
func (c *AIChatCoordinator) EndSession(chatID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if session, exists := c.sessions[chatID]; exists {
		session.isActive = false
		c.logger.Info("AI chat session ended: chat=%s", chatID)
	}
}

// IsMessageFromParticipantBot checks if a message is from a bot that's part of an active session.
func (c *AIChatCoordinator) IsMessageFromParticipantBot(chatID, senderID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, exists := c.sessions[chatID]
	if !exists || !session.isActive {
		return false
	}

	for _, participant := range session.participants {
		if participant == senderID {
			return true
		}
	}
	return false
}

// GetSessionInfo returns information about an active session for debugging.
func (c *AIChatCoordinator) GetSessionInfo(chatID string) (info string, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	session, ok := c.sessions[chatID]
	if !ok {
		return "", false
	}

	info = fmt.Sprintf("chat=%s participants=%v turn=%d/%d messages=%d/%d active=%t",
		session.chatID, session.participants, session.currentTurn+1, len(session.participants),
		session.messageCount, session.maxMessages, session.isActive)
	return info, true
}

// CleanupExpiredSessions removes sessions that haven't had activity for the given duration.
func (c *AIChatCoordinator) CleanupExpiredSessions(maxAge time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0
	for chatID, session := range c.sessions {
		if now.Sub(session.lastActivity) > maxAge {
			delete(c.sessions, chatID)
			removed++
		}
	}
	return removed
}

// extractMentionedBots filters mentions to only include known bot IDs.
func (c *AIChatCoordinator) extractMentionedBots(mentions []string) []string {
	var bots []string
	for _, mention := range mentions {
		if c.botIDs[mention] {
			bots = append(bots, mention)
		}
	}
	return bots
}

// orderParticipants determines the order in which bots should respond.
// Currently uses a simple deterministic order based on ID sorting.
func (c *AIChatCoordinator) orderParticipants(bots []string, thisBotID string) []string {
	// Sort for deterministic ordering
	ordered := make([]string, len(bots))
	copy(ordered, bots)
	sort.Strings(ordered)
	return ordered
}

// IsBotID checks if the given ID is a registered bot.
func (c *AIChatCoordinator) IsBotID(id string) bool {
	return c.botIDs[id]
}
