package metrics

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/push"
)

type Pusher struct {
	*push.Pusher
}

func NewPusher(metricsURL, metricsToken string) *Pusher {
	if metricsURL == "" || metricsToken == "" {
		return &Pusher{}
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	token := base64.StdEncoding.EncodeToString([]byte(metricsToken))
	return &Pusher{
		push.New(metricsURL, "dmt").Client(httpClient).
			Header(http.Header{"Authorization": []string{"Bearer " + token}}),
	}
}

func (p *Pusher) Push() error {
	if p.Pusher == nil {
		return nil
	}

	return p.Pusher.Push()
}
