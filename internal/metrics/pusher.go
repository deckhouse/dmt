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

func NewPusher(metricsUrl, metricsToken string) *push.Pusher {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	token := base64.StdEncoding.EncodeToString([]byte(metricsToken))
	return push.New(metricsUrl, "dmt").Client(httpClient).
		Header(http.Header{"Authorization": []string{"Bearer " + token}})
}
