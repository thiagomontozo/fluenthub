package liveclasses

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// LiveKitProvider uses LiveKit's authenticated Twirp API and issues short-lived
// participant tokens. It does not expose API credentials to the browser.
type LiveKitProvider struct {
	baseURL, apiKey, apiSecret string
	client                     *http.Client
	mu                         sync.Mutex
	egressByRoom               map[string]string
}

func (*LiveKitProvider) Name() string { return "livekit" }

func NewLiveKitProvider(rawURL, apiKey, apiSecret string, client *http.Client) (*LiveKitProvider, error) {
	if apiKey == "" || len(apiSecret) < 16 {
		return nil, errors.New("livekit API credentials are incomplete")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return nil, errors.New("invalid LiveKit URL")
	}
	if parsed.Scheme == "ws" {
		parsed.Scheme = "http"
	} else if parsed.Scheme == "wss" {
		parsed.Scheme = "https"
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1")) {
		return nil, errors.New("LiveKit API must use HTTPS outside localhost")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &LiveKitProvider{baseURL: strings.TrimRight(parsed.String(), "/"), apiKey: apiKey, apiSecret: apiSecret, client: client, egressByRoom: map[string]string{}}, nil
}

func (p *LiveKitProvider) CreateSession(ctx context.Context, lessonID string) (Session, error) {
	room := "fluenthub-" + lessonID
	var response struct {
		Name string `json:"name"`
	}
	if err := p.call(ctx, "livekit.RoomService", "CreateRoom", map[string]any{"name": room, "emptyTimeout": 600, "maxParticipants": 250}, &response); err != nil {
		return Session{}, err
	}
	if response.Name == "" {
		response.Name = room
	}
	return Session{LessonID: lessonID, Provider: "livekit", ProviderSessionID: response.Name, Status: "created"}, nil
}

func (p *LiveKitProvider) StartSession(_ context.Context, session Session) (Session, error) {
	now := time.Now().UTC()
	session.StartedAt, session.Status = &now, "live"
	return session, nil
}

func (p *LiveKitProvider) EndSession(ctx context.Context, session Session) (Session, error) {
	if err := p.call(ctx, "livekit.RoomService", "DeleteRoom", map[string]string{"room": session.ProviderSessionID}, nil); err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	session.EndedAt, session.Status = &now, "ended"
	return session, nil
}

func (p *LiveKitProvider) GetTeacherJoinInfo(_ context.Context, session Session, userID string) (JoinInfo, error) {
	return p.joinInfo(session, userID, true)
}

func (p *LiveKitProvider) GetStudentJoinInfo(_ context.Context, session Session, userID string) (JoinInfo, error) {
	return p.joinInfo(session, userID, false)
}

func (p *LiveKitProvider) joinInfo(session Session, userID string, teacher bool) (JoinInfo, error) {
	expires := time.Now().UTC().Add(15 * time.Minute)
	grant := map[string]any{"roomJoin": true, "room": session.ProviderSessionID, "canSubscribe": true, "canPublish": teacher, "canPublishData": true}
	token, err := p.token(userID, expires, grant)
	if err != nil {
		return JoinInfo{}, err
	}
	return JoinInfo{URL: p.baseURL, Token: token, ExpiresAt: expires}, nil
}

// Recording uses LiveKit Egress. The deployment must configure its Egress
// service output independently; the API request deliberately contains no cloud
// credentials.
func (p *LiveKitProvider) StartRecording(ctx context.Context, session Session) error {
	var response struct {
		EgressID string `json:"egressId"`
	}
	request := map[string]any{"roomName": session.ProviderSessionID, "layout": "speaker", "preset": "HD_720P_30", "fileOutputs": []map[string]any{{"filepath": "fluenthub/recordings/{room_name}-{time}.mp4"}}}
	if err := p.call(ctx, "livekit.Egress", "StartRoomCompositeEgress", request, &response); err != nil {
		return err
	}
	if response.EgressID == "" {
		return errors.New("LiveKit Egress returned no egress ID")
	}
	p.mu.Lock()
	p.egressByRoom[session.ProviderSessionID] = response.EgressID
	p.mu.Unlock()
	return nil
}

func (p *LiveKitProvider) StopRecording(ctx context.Context, session Session) error {
	p.mu.Lock()
	id := p.egressByRoom[session.ProviderSessionID]
	p.mu.Unlock()
	if id == "" {
		return errors.New("no active egress for room")
	}
	if err := p.call(ctx, "livekit.Egress", "StopEgress", map[string]string{"egressId": id}, nil); err != nil {
		return err
	}
	p.mu.Lock()
	delete(p.egressByRoom, session.ProviderSessionID)
	p.mu.Unlock()
	return nil
}

func (p *LiveKitProvider) call(ctx context.Context, service, method string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(2 * time.Minute)
	token, err := p.token("fluenthub-api", expires, map[string]any{"roomCreate": true, "roomAdmin": true, "roomRecord": true})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/twirp/"+service+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("LiveKit %s request failed: %w", method, err)
	}
	defer response.Body.Close()
	limited, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("LiveKit %s returned HTTP %d", method, response.StatusCode)
	}
	if output != nil && len(limited) > 0 && string(limited) != "{}" {
		if err := json.Unmarshal(limited, output); err != nil {
			return fmt.Errorf("decode LiveKit %s response: %w", method, err)
		}
	}
	return nil
}

func (p *LiveKitProvider) token(subject string, expires time.Time, video map[string]any) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	now := time.Now().UTC()
	payload, err := json.Marshal(map[string]any{"iss": p.apiKey, "sub": subject, "nbf": now.Add(-5 * time.Second).Unix(), "exp": expires.Unix(), "video": video})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(p.apiSecret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
