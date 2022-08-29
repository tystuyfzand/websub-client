package client

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"io"
	"log"
	"meow.tf/websub/handler"
	"meow.tf/websub/model"
	"meow.tf/websub/store"
	"meow.tf/websub/store/memory"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	formTag         = "form"
	formContentType = "application/x-www-form-urlencoded"
)

var (
	ErrNoHub        = errors.New("hub not found")
	ErrNoSelf       = errors.New("self link not found")
	ErrNoChallenge  = errors.New("no challenge specified")
	ErrInvalidLease = errors.New("invalid lease duration")

	DefaultLease = 24 * time.Hour
)

// Option is an option type for definition client options
type Option func(c *Client)

// WithStore sets the Client's subscription store
func WithStore(s store.Store) Option {
	return func(c *Client) {
		c.store = s
	}
}

// WithCallbackBase sets the base callback url
func WithCallbackBase(base string) Option {
	return func(c *Client) {
		c.callbackBase = base
	}
}

// WithHttpClient lets you override the client's http client
func WithHttpClient(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}

// WithLeaseDuration lets you override the default lease duration request
func WithLeaseDuration(duration time.Duration) Option {
	return func(c *Client) {
		c.leaseDuration = duration
	}
}

// New creates a new client with the specified callback base url and options
func New(callbackBase string, options ...Option) *Client {
	c := &Client{
		Handler: handler.New(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		callbackBase:  callbackBase,
		leaseDuration: DefaultLease,
		validator:     validator.New(),
		store:         memory.New(),
	}

	for _, opt := range options {
		opt(c)
	}

	return c
}

// Client represents a websub client
type Client struct {
	*handler.Handler
	client        *http.Client
	store         store.Store
	callbackBase  string
	leaseDuration time.Duration
	validator     *validator.Validate

	pendingSubscribes   []model.Subscription
	pendingUnsubscribes []model.Subscription
}

// SubscribeOptions are options sent to the hub for subscriptions
type SubscribeOptions struct {
	Hub      string
	Topic    string
	Callback string
	Secret   string
	Lease    time.Duration
}

// Subscribe creates a new websub request
func (c *Client) Subscribe(opts SubscribeOptions) (*model.Subscription, error) {
	topicURL := opts.Topic
	var hubURL string

	if opts.Hub == "" {
		var err error

		topicURL, hubURL, err = c.Discover(opts.Topic)

		if err != nil {
			return nil, err
		}
	} else {
		hubURL = opts.Hub
	}

	if opts.Callback == "" && c.callbackBase != "" {
		subHash := sha256.New().Sum([]byte(topicURL))

		opts.Callback = c.callbackBase + "/" + hex.EncodeToString(subHash)
	}

	u, err := url.Parse(opts.Callback)

	if err != nil {
		return nil, err
	}

	if c.callbackBase == "" {
		c.callbackBase = u.Scheme + "://" + u.Host
	}

	subscribeReq := model.SubscribeRequest{
		Mode:         model.ModeSubscribe,
		Topic:        topicURL,
		Callback:     opts.Callback,
		Secret:       opts.Secret,
		LeaseSeconds: int(c.leaseDuration / time.Second),
	}

	if opts.Lease > 0 {
		subscribeReq.LeaseSeconds = int(opts.Lease / time.Second)
	}

	if err = c.validator.Struct(subscribeReq); err != nil {
		return nil, err
	}

	sub, err := c.store.Get(topicURL, subscribeReq.Callback)

	if sub == nil {
		sub = &model.Subscription{
			Topic:    topicURL,
			Callback: opts.Callback,
		}
	}

	err = c.store.Add(*sub)

	if err != nil {
		return nil, err
	}

	c.pendingSubscribes = append(c.pendingSubscribes, *sub)

	body := encodeForm(subscribeReq)

	req, err := http.NewRequest(http.MethodPost, hubURL, strings.NewReader(body))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", formContentType)

	res, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected response code %d: %s", res.StatusCode, string(b))
	}

	return sub, nil
}

// Unsubscribe sends an unsubscription request to the hub
func (c *Client) Unsubscribe(unsubscribeReq model.UnsubscribeRequest) error {
	topicURL, hubUrl, err := c.Discover(unsubscribeReq.Topic)

	if err != nil {
		return err
	}

	sub, err := c.store.Get(topicURL, unsubscribeReq.Callback)

	if err != nil {
		return err
	}

	c.pendingUnsubscribes = append(c.pendingUnsubscribes, *sub)

	body := encodeForm(unsubscribeReq)

	req, err := http.NewRequest(http.MethodPost, hubUrl, strings.NewReader(body))

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", formContentType)

	res, err := c.client.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected response code %d: %s", res.StatusCode, string(b))
	}

	return nil
}

// VerifySubscription lets other handlers pass subscriptions, unsubscribes, and denied errors themselves.
func (c *Client) VerifySubscription(mode, topic, requestUrl string, v url.Values) ([]byte, error) {
	switch mode {
	case model.ModeSubscribe:
		challenge := v.Get("hub.challenge")

		if challenge == "" {
			return nil, ErrNoChallenge
		}

		var sub *model.Subscription

		for i, s := range c.pendingSubscribes {
			if s.Topic == topic && s.Callback == requestUrl {
				sub = &s

				c.pendingSubscribes = remove(c.pendingSubscribes, i)
				break
			}
		}

		if sub == nil {
			return nil, store.ErrNotFound
		}

		leaseSeconds, err := strconv.Atoi(v.Get("hub.lease_seconds"))

		if err != nil {
			return nil, ErrInvalidLease
		}

		sub.LeaseTime = time.Duration(leaseSeconds) * time.Second
		sub.Expires = time.Now().Add(sub.LeaseTime)

		err = c.store.Add(*sub)

		if err != nil {
			return nil, err
		}

		return []byte(challenge), nil
	case model.ModeDenied:
		var sub *model.Subscription

		for i, s := range c.pendingSubscribes {
			if s.Topic == topic && s.Callback == requestUrl {
				sub = &s

				c.pendingSubscribes = remove(c.pendingSubscribes, i)
				break
			}
		}

		if sub == nil {
			return nil, store.ErrNotFound
		}

		return nil, fmt.Errorf("subscription denied: %s", v.Get("hub.reason"))
	case model.ModeUnsubscribe:
		challenge := v.Get("hub.challenge")

		if challenge == "" {
			return nil, ErrNoChallenge
		}

		var sub *model.Subscription

		for i, s := range c.pendingUnsubscribes {
			if s.Topic == topic {
				sub = &s

				c.pendingUnsubscribes = remove(c.pendingUnsubscribes, i)
				break
			}
		}

		err := c.store.Remove(*sub)

		if err != nil {
			return nil, err
		}

		return []byte(challenge), nil
	}

	return nil, nil
}

// ServeHTTP lets this client be used as a handler for HTTP servers.
func (c *Client) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v, err := url.ParseQuery(r.URL.RawQuery)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	requestUrl := c.callbackBase + r.RequestURI

	if idx := strings.Index(requestUrl, "?"); idx != -1 {
		requestUrl = requestUrl[0:idx]
	}

	mode := v.Get("hub.mode")
	topic := v.Get("hub.topic")

	if mode != "" {
		res, err := c.VerifySubscription(mode, topic, requestUrl, v)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if res != nil {
			w.Write(res)
		}

		return
	}

	log.Println("Request that doesn't have a mode")

	// Verify subscription exists
	subs, err := c.store.For(requestUrl)

	if err != nil || len(subs) < 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	hubSignature := r.Header.Get("X-Hub-Signature")

	sub := subs[0]

	b, err := io.ReadAll(r.Body)

	if err != nil {
		return
	}

	if sub.Secret != "" {
		if hubSignature == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if !ValidateSignature(b, sub.Secret, hubSignature) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	c.Call(&Publish{
		Subscription: sub,
		ContentType:  r.Header.Get("Content-Type"),
		Data:         b,
	})
}

// ValidateSignature is a helper method to validate a signature a hub sends.
func ValidateSignature(body []byte, secret, signature string) bool {
	splitIdx := strings.Index(signature, "=")

	if splitIdx == -1 {
		return false
	}

	hasher := signature[0:splitIdx]

	signature = signature[splitIdx+1:]

	mac := hmac.New(newHash(hasher), []byte(secret))
	mac.Write(body)

	return signature == hex.EncodeToString(mac.Sum(nil))
}
