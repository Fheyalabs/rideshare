package ghost

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is a minimal HTTP client for the rideshare server API. Ghosts
// and test riders share this — it hits the same endpoints as real phones.
type Client struct{ Base string }

// NewClient returns a Client for the given server URL.
func NewClient(base string) *Client {
	return &Client{Base: strings.TrimRight(base, "/")}
}

func (c *Client) get(path string) ([]byte, error) {
	resp, err := http.Get(c.Base + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", path, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Post sends a JSON POST and returns the raw response body.
func (c *Client) Post(path string, body any) ([]byte, error) {
	return c.post(path, body)
}

func (c *Client) post(path string, body any) ([]byte, error) {
	b, _ := json.Marshal(body)
	resp, err := http.Post(c.Base+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return data, fmt.Errorf("%s: HTTP %d: %s", path, resp.StatusCode, string(data))
	}
	return data, nil
}

// PutArtifact uploads data and returns the hex handle.
func (c *Client) PutArtifact(data []byte) (string, error) {
	resp, err := c.post("/artifacts", map[string]string{"Data": base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		return "", err
	}
	var out struct{ Handle string }
	json.Unmarshal(resp, &out)
	if out.Handle == "" {
		return "", fmt.Errorf("empty handle")
	}
	return out.Handle, nil
}

// GetArtifact downloads data by handle.
func (c *Client) GetArtifact(handle string) ([]byte, error) {
	return c.get("/artifacts/" + handle)
}

// PushGrid sends a cell update.
func (c *Client) PushGrid(pseudonym, cell string) error {
	_, err := c.post("/grid", map[string]any{
		"pseudonym": pseudonym, "cell": cell, "accepting": true,
	})
	return err
}

// Invites returns pending invites for a pseudonym.
func (c *Client) Invites(pseudonym string) ([]Invite, error) {
	data, err := c.get("/invites/" + pseudonym)
	if err != nil {
		return nil, err
	}
	var out struct{ Invites []Invite }
	json.Unmarshal(data, &out)
	return out.Invites, nil
}

// SubmitBid posts a signed encrypted bid.
func (c *Client) SubmitBid(sessionID, bidHandle string, nonce, pubkey, sig []byte) error {
	_, err := c.post("/session/bid", map[string]any{
		"session_id": sessionID, "bid_handle": bidHandle,
		"nonce": nonce, "pubkey": pubkey, "sig": sig,
	})
	return err
}

// OpenSession creates a ride session and returns candidate list.
func (c *Client) OpenSession(req map[string]any) ([]string, error) {
	data, err := c.post("/session/open", req)
	if err != nil {
		return nil, err
	}
	var out struct{ Candidates []string `json:"candidates"` }
	json.Unmarshal(data, &out)
	return out.Candidates, nil
}

// GetMasks returns the mask handles for a session.
func (c *Client) GetMasks(sessionID string) ([]string, error) {
	data, err := c.get("/session/" + sessionID + "/masks")
	if err != nil {
		return nil, fmt.Errorf("%w (body: %s)", err, string(data))
	}
	var out struct{ MaskHandles []string `json:"mask_handles"` }
	json.Unmarshal(data, &out)
	return out.MaskHandles, nil
}
