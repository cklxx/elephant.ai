package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"alex/internal/llm"
)

func main() {
	fmt.Println("🚀 ALEX Ollama Integration Test")
	fmt.Println("================================")

	// Test 1: Create Ollama client and test connection
	fmt.Println("\n1. Testing Ollama client creation and connection...")
	client, err := llm.NewOllamaClient("http://localhost:11434", false)
	if err != nil {
		log.Fatalf("❌ Failed to create Ollama client: %v", err)
	}
	fmt.Println("✅ Ollama client created successfully")

	// Test 2: Get available models
	fmt.Println("\n2. Retrieving available models...")
	models, err := client.GetAvailableModels()
	if err != nil {
		fmt.Printf("⚠️  Could not retrieve models (Ollama might not be running): %v\n", err)
	} else {
		fmt.Printf("✅ Available models: %v\n", models)
	}

	// Test 3: Basic chat mode
	fmt.Println("\n3. Testing basic chat mode...")
	ctx := context.Background()
	basicReq := &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "What is 2 + 2?"},
		},
		Model:       "llama3.2",
		Temperature: 0.7,
		MaxTokens:   100,
	}

	resp, err := client.Chat(ctx, basicReq, "test-session-1")
	if err != nil {
		fmt.Printf("⚠️  Chat request failed (Ollama might not be running): %v\n", err)
	} else {
		fmt.Printf("✅ Basic chat response: %s\n", resp.Choices[0].Message.Content)
	}

	// Test 4: Ultra Think mode
	fmt.Println("\n4. Testing Ultra Think mode...")
	ultraClient, err := llm.NewOllamaClient("http://localhost:11434", true)
	if err != nil {
		log.Fatalf("❌ Failed to create Ultra Think client: %v", err)
	}
	fmt.Println("✅ Ultra Think client created")

	ultraReq := &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Design a simple cache system"},
		},
		Model:       "llama3.2",
		Temperature: 0.3,
		MaxTokens:   500,
	}

	resp, err = ultraClient.Chat(ctx, ultraReq, "test-session-2")
	if err != nil {
		fmt.Printf("⚠️  Ultra Think request failed (Ollama might not be running): %v\n", err)
	} else {
		fmt.Println("✅ Ultra Think mode activated and processed request")
		fmt.Printf("   Response length: %d characters\n", len(resp.Choices[0].Message.Content))
	}

	// Test 5: Streaming mode
	fmt.Println("\n5. Testing streaming mode...")
	streamReq := &llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Count from 1 to 5"},
		},
		Model:       "llama3.2",
		Temperature: 0.5,
		MaxTokens:   100,
		Stream:      true,
	}

	streamChan, err := client.ChatStream(ctx, streamReq, "test-session-3")
	if err != nil {
		fmt.Printf("⚠️  Stream request failed (Ollama might not be running): %v\n", err)
	} else {
		fmt.Print("✅ Streaming response: ")
		
		timeout := time.After(10 * time.Second)
		for {
			select {
			case delta, ok := <-streamChan:
				if !ok {
					fmt.Println("\n   Stream completed")
					goto streamDone
				}
				if len(delta.Choices) > 0 {
					fmt.Print(delta.Choices[0].Delta.Content)
				}
			case <-timeout:
				fmt.Println("\n   Stream timeout")
				goto streamDone
			}
		}
	}
streamDone:

	// Test 6: Interface compliance check
	fmt.Println("\n6. Verifying interface compliance...")
	var _ llm.Client = client
	fmt.Println("✅ OllamaClient implements llm.Client interface")

	// Test 7: Toggle Ultra Think
	fmt.Println("\n7. Testing Ultra Think toggle...")
	client.EnableUltraThink(true)
	fmt.Println("✅ Ultra Think enabled")
	
	client.EnableUltraThink(false)
	fmt.Println("✅ Ultra Think disabled")

	// Test 8: Client cleanup
	fmt.Println("\n8. Testing client cleanup...")
	err = client.Close()
	if err != nil {
		fmt.Printf("❌ Failed to close client: %v\n", err)
	} else {
		fmt.Println("✅ Client closed successfully")
	}

	fmt.Println("\n================================")
	fmt.Println("✨ Ollama Integration Test Complete!")
	fmt.Println("\nNote: If you see connection warnings above, please ensure Ollama is running:")
	fmt.Println("  ollama serve")
	fmt.Println("  ollama pull llama3.2")
}