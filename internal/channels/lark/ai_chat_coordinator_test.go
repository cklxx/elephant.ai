package lark

import (
	"testing"
	"time"

	"alex/internal/logging"
)

func TestNewAIChatCoordinator(t *testing.T) {
	logger := logging.OrNop(nil)
	botIDs := []string{"bot1", "bot2", "bot3"}
	
	coord := NewAIChatCoordinator(logger, botIDs)
	if coord == nil {
		t.Fatal("NewAIChatCoordinator returned nil")
	}
	
	// Test IsBotID
	if !coord.IsBotID("bot1") {
		t.Error("IsBotID should return true for registered bot")
	}
	if coord.IsBotID("bot4") {
		t.Error("IsBotID should return false for unregistered bot")
	}
}

func TestAIChatCoordinator_DetectAndStartSession(t *testing.T) {
	logger := logging.OrNop(nil)
	coord := NewAIChatCoordinator(logger, []string{"bot1", "bot2", "bot3"})
	
	tests := []struct {
		name           string
		mentions       []string
		thisBotID      string
		wantParticipate bool
		wantWait       bool
	}{
		{
			name:           "single bot mention - not multi-bot",
			mentions:       []string{"bot1"},
			thisBotID:      "bot1",
			wantParticipate: false,
			wantWait:       false,
		},
		{
			name:           "multi-bot mention - first bot's turn",
			mentions:       []string{"bot1", "bot2"},
			thisBotID:      "bot1",
			wantParticipate: true,
			wantWait:       false,
		},
		{
			name:           "multi-bot mention - second bot waits",
			mentions:       []string{"bot1", "bot2"},
			thisBotID:      "bot2",
			wantParticipate: true,
			wantWait:       true,
		},
		{
			name:           "bot not mentioned",
			mentions:       []string{"bot1", "bot2"},
			thisBotID:      "bot3",
			wantParticipate: false,
			wantWait:       false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotParticipate, gotWait := coord.DetectAndStartSession(
				"chat_123", "msg_456", "user_789", tt.mentions, tt.thisBotID,
			)
			if gotParticipate != tt.wantParticipate {
				t.Errorf("DetectAndStartSession() participate = %v, want %v", gotParticipate, tt.wantParticipate)
			}
			if gotWait != tt.wantWait {
				t.Errorf("DetectAndStartSession() wait = %v, want %v", gotWait, tt.wantWait)
			}
		})
	}
}

func TestAIChatCoordinator_TurnTaking(t *testing.T) {
	logger := logging.OrNop(nil)
	coord := NewAIChatCoordinator(logger, []string{"bot1", "bot2"})
	
	chatID := "chat_test"
	
	// Start a session
	mentions := []string{"bot1", "bot2"}
	coord.DetectAndStartSession(chatID, "msg_1", "user_1", mentions, "bot1")
	
	// Check initial turn - should be bot1's turn
	if !coord.ShouldBotRespond(chatID, "bot1") {
		t.Error("ShouldBotRespond should return true for bot1 initially")
	}
	if coord.ShouldBotRespond(chatID, "bot2") {
		t.Error("ShouldBotRespond should return false for bot2 initially")
	}
	
	// Advance turn from bot1
	nextBot, shouldContinue := coord.AdvanceTurn(chatID, "bot1")
	if !shouldContinue {
		t.Error("AdvanceTurn should return shouldContinue=true")
	}
	if nextBot != "bot2" {
		t.Errorf("AdvanceTurn nextBot = %s, want bot2", nextBot)
	}
	
	// Now should be bot2's turn
	if coord.ShouldBotRespond(chatID, "bot1") {
		t.Error("ShouldBotRespond should return false for bot1 after advance")
	}
	if !coord.ShouldBotRespond(chatID, "bot2") {
		t.Error("ShouldBotRespond should return true for bot2 after advance")
	}
	
	// Advance turn from bot2
	nextBot, shouldContinue = coord.AdvanceTurn(chatID, "bot2")
	if !shouldContinue {
		t.Error("AdvanceTurn should return shouldContinue=true")
	}
	if nextBot != "bot1" {
		t.Errorf("AdvanceTurn nextBot = %s, want bot1 (cycled back)", nextBot)
	}
}

func TestAIChatCoordinator_MessageLimit(t *testing.T) {
	logger := logging.OrNop(nil)
	coord := NewAIChatCoordinator(logger, []string{"bot1", "bot2"})
	
	chatID := "chat_limit_test"
	mentions := []string{"bot1", "bot2"}
	
	// Start session
	coord.DetectAndStartSession(chatID, "msg_1", "user_1", mentions, "bot1")
	
	// Advance turn multiple times to hit the limit (default is 10)
	for i := 0; i < 10; i++ {
		bot := "bot1"
		if i%2 == 1 {
			bot = "bot2"
		}
		nextBot, shouldContinue := coord.AdvanceTurn(chatID, bot)
		if i < 9 && !shouldContinue {
			t.Errorf("AdvanceTurn should continue at iteration %d", i)
		}
		if i == 9 && shouldContinue {
			t.Error("AdvanceTurn should stop at iteration 9 (limit reached)")
		}
		if i == 9 && nextBot != "" {
			t.Error("AdvanceTurn should return empty nextBot when limit reached")
		}
	}
}

func TestAIChatCoordinator_IsMessageFromParticipantBot(t *testing.T) {
	logger := logging.OrNop(nil)
	coord := NewAIChatCoordinator(logger, []string{"bot1", "bot2"})
	
	chatID := "chat_participant_test"
	mentions := []string{"bot1", "bot2"}
	
	// Before session starts
	if coord.IsMessageFromParticipantBot(chatID, "bot1") {
		t.Error("IsMessageFromParticipantBot should return false before session starts")
	}
	
	// Start session
	coord.DetectAndStartSession(chatID, "msg_1", "user_1", mentions, "bot1")
	
	// Check participant
	if !coord.IsMessageFromParticipantBot(chatID, "bot1") {
		t.Error("IsMessageFromParticipantBot should return true for participant")
	}
	if coord.IsMessageFromParticipantBot(chatID, "bot3") {
		t.Error("IsMessageFromParticipantBot should return false for non-participant")
	}
}

func TestAIChatCoordinator_EndSession(t *testing.T) {
	logger := logging.OrNop(nil)
	coord := NewAIChatCoordinator(logger, []string{"bot1", "bot2"})
	
	chatID := "chat_end_test"
	mentions := []string{"bot1", "bot2"}
	
	// Start and end session
	coord.DetectAndStartSession(chatID, "msg_1", "user_1", mentions, "bot1")
	coord.EndSession(chatID)
	
	// After ending, should not respond
	if coord.ShouldBotRespond(chatID, "bot1") {
		t.Error("ShouldBotRespond should return false after session ended")
	}
}

func TestAIChatCoordinator_CleanupExpiredSessions(t *testing.T) {
	logger := logging.OrNop(nil)
	coord := NewAIChatCoordinator(logger, []string{"bot1", "bot2"})
	
	chatID := "chat_cleanup_test"
	mentions := []string{"bot1", "bot2"}
	
	// Start session
	coord.DetectAndStartSession(chatID, "msg_1", "user_1", mentions, "bot1")
	
	// Cleanup with very short max age should remove the session
	removed := coord.CleanupExpiredSessions(1 * time.Nanosecond)
	if removed != 1 {
		t.Errorf("CleanupExpiredSessions removed = %d, want 1", removed)
	}
	
	// Session should be gone
	if coord.ShouldBotRespond(chatID, "bot1") {
		t.Error("ShouldBotRespond should return false after cleanup")
	}
}

func TestAIChatCoordinator_GetSessionInfo(t *testing.T) {
	logger := logging.OrNop(nil)
	coord := NewAIChatCoordinator(logger, []string{"bot1", "bot2"})
	
	// Non-existent session
	_, exists := coord.GetSessionInfo("non_existent")
	if exists {
		t.Error("GetSessionInfo should return exists=false for non-existent session")
	}
	
	// Create session
	chatID := "chat_info_test"
	mentions := []string{"bot1", "bot2"}
	coord.DetectAndStartSession(chatID, "msg_1", "user_1", mentions, "bot1")
	
	info, exists := coord.GetSessionInfo(chatID)
	if !exists {
		t.Error("GetSessionInfo should return exists=true for active session")
	}
	if info == "" {
		t.Error("GetSessionInfo should return non-empty info")
	}
}
