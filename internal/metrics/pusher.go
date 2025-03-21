/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
