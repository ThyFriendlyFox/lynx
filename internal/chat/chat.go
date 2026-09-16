// Package chat answers questions about the files in a shared folder.
package chat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrNoProvider reports a chat with no way to reach a model.
var ErrNoProvider = errors.New("lynx chat needs an API key")

// Message is one turn in a conversation.
type Message struct {
	Role string `json:"role"` // "user" or "assistant"
	Text string `json:"text"`
}

// Request is what a provider answers.
type Request struct {
	System   string
	Messages []Message
}

// Provider sends a request to a model and writes the answer.
type Provider interface {
	// Name is the value of the provider field in the config.
	Name() string
	// Answer writes the answer to w while it arrives and returns the whole
	// answer.
	Answer(ctx context.Context, req Request, w io.Writer) (string, error)
}

// Session holds one conversation about one folder.
type Session struct {
	provider Provider
	system   string
	history  []Message
}

// NewSession starts a conversation about a folder.
func NewSession(p Provider, folderContext string) *Session {
	return &Session{provider: p, system: folderContext}
}

// Ask sends one question and writes the answer to w.
func (s *Session) Ask(ctx context.Context, question string, w io.Writer) (string, error) {
	s.history = append(s.history, Message{Role: "user", Text: question})
	answer, err := s.provider.Answer(ctx, Request{System: s.system, Messages: s.history}, w)
	if err != nil {
		// A failed turn does not stay in the history, or every later turn
		// repeats the failure.
		s.history = s.history[:len(s.history)-1]
		return "", err
	}
	s.history = append(s.history, Message{Role: "assistant", Text: answer})
	return answer, nil
}

// Turns returns how many questions the session holds.
func (s *Session) Turns() int { return len(s.history) / 2 }

// Loop reads questions from in and writes answers to out until the reader
// ends or the person types "exit".
func (s *Session) Loop(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for {
		fmt.Fprint(out, "\nyou: ")
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return scanner.Err()
		}
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "exit" || question == "quit" {
			return nil
		}
		fmt.Fprint(out, "\nfolder: ")
		if _, err := s.Ask(ctx, question, out); err != nil {
			fmt.Fprintf(out, "\nlynx: %s\n", err)
		}
		fmt.Fprintln(out)
	}
}
