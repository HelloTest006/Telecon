package signal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client talks to the HTTP signaling server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	From    string
}

// NewClient creates a signaling client.
func NewClient(baseURL, from string) *Client {
	return &Client{
		BaseURL: baseURL,
		From:    from,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Send posts an envelope.
func (c *Client) Send(env Envelope) error {
	env.From = c.From
	b, _ := json.Marshal(env)
	resp, err := c.HTTP.Post(c.BaseURL+"/v1/signal/send", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("signal send %d: %s", resp.StatusCode, body)
	}
	return nil
}

// Poll waits for messages.
func (c *Client) Poll(room string, wait time.Duration) ([]Envelope, error) {
	u, _ := url.Parse(c.BaseURL + "/v1/signal/poll")
	q := u.Query()
	q.Set("room", room)
	q.Set("from", c.From)
	q.Set("wait", wait.String())
	u.RawQuery = q.Encode()
	resp, err := c.HTTP.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Messages []Envelope `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Messages, nil
}

// WaitType polls until a message of type arrives or timeout.
func (c *Client) WaitType(room, typ string, overall time.Duration) (Envelope, error) {
	deadline := time.Now().Add(overall)
	for time.Now().Before(deadline) {
		remain := time.Until(deadline)
		if remain > 20*time.Second {
			remain = 20 * time.Second
		}
		msgs, err := c.Poll(room, remain)
		if err != nil {
			return Envelope{}, err
		}
		for _, m := range msgs {
			if m.Type == typ {
				return m, nil
			}
			// return ice to caller via multi — for simplicity re-queue not available;
			// WebRTC layer should Poll in loop and handle all types.
		}
		if len(msgs) > 0 {
			// return first non-matching by packing — actually return error with batch
			// Better: expose Poll only. WaitType only for offer/answer.
			for _, m := range msgs {
				if m.Type == typ {
					return m, nil
				}
			}
		}
	}
	return Envelope{}, fmt.Errorf("timeout waiting for %s", typ)
}
