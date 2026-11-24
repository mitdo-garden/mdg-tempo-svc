package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendUsesGivenChat(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sendMessage") {
			t.Fatalf("path %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New("super-secret-token")
	c.base = srv.URL
	c.http = srv.Client()
	if err := c.Send(context.Background(), "4242", "Phở\nfood"); err != nil {
		t.Fatal(err)
	}
	if got["chat_id"] != "4242" || got["text"] != "Phở\nfood" {
		t.Fatalf("body %#v", got)
	}
}

func TestReplyChatIDsAnswersStart(t *testing.T) {
	var offset float64
	var sawOffset bool
	var reply string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if v, ok := body["offset"]; ok {
				sawOffset = true
				offset = v.(float64)
				_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":[
				{"update_id":7,"message":{"text":"/start","chat":{"id":555,"type":"private"}}},
				{"update_id":8,"message":{"text":"hello","chat":{"id":555,"type":"private"}}}
			]}`))
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["chat_id"] != "555" {
				t.Fatalf("chat %s", body["chat_id"])
			}
			reply = body["text"]
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New("tok")
	c.base = srv.URL
	c.http = srv.Client()
	if err := c.ReplyChatIDs(context.Background()); err != nil {
		t.Fatal(err)
	}
	if reply != "Your chat id is 555" {
		t.Fatalf("reply %q", reply)
	}
	if err := c.ReplyChatIDs(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !sawOffset || offset != 9 {
		t.Fatalf("offset %v saw %v", offset, sawOffset)
	}
}

func TestReplyChatIDsClaimsStartCode(t *testing.T) {
	var reply string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			_, _ = w.Write([]byte(`{"ok":true,"result":[
				{"update_id":3,"message":{"text":"/start garden1","chat":{"id":777,"type":"private"}}}
			]}`))
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			reply = body["text"]
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New("tok")
	c.base = srv.URL
	c.http = srv.Client()
	var gotCode, gotChat string
	c.OnClaim(func(_ context.Context, chatID, code string) (bool, error) {
		gotChat, gotCode = chatID, code
		return true, nil
	})
	if err := c.ReplyChatIDs(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotChat != "777" || gotCode != "garden1" {
		t.Fatalf("claim %s %s", gotChat, gotCode)
	}
	if reply != "Connected. Your chat id is 777" {
		t.Fatalf("reply %q", reply)
	}
}

func TestNetworkErrorHidesToken(t *testing.T) {
	c := New("super-secret-token")
	c.base = "http://127.0.0.1:1"
	c.http = &http.Client{}
	err := c.Send(context.Background(), "1", "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "super-secret-token") {
		t.Fatalf("token leaked: %v", err)
	}
}
