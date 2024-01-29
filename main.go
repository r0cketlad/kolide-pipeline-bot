// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Create Slack notifications for incoming osquery/Kolide events
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/VirusTotal/vt-go"
	"github.com/slack-go/slack"
	"k8s.io/klog/v2"
)

var (
	bucketFlag         = flag.String("bucket", "", "Bucket to query")
	prefixFlag         = flag.String("prefix", "", "directory of contents to query")
	excludeSubDirsFlag = flag.String("exclude-subdirs", "", "exclude alerts for this comma-separated list of subdirectories")
	channelFlag        = flag.String("channel-id", "", "Slack channel to post to (required for replies)")
	serveFlag          = flag.Bool("serve", false, "")
	maxAgeFlag         = flag.Duration("max-age", 30*time.Minute, "Maximum age of events to include (for best use, use at least 2X your trigger time)")
	maxNoticesFlag     = flag.Int("max-notices-per-kind", 5, "Maximum notices per kind (spam reduction)")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	ctx := context.Background()

	cutoff := time.Now().Add(*maxAgeFlag * -1)

	// Creates a client.
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	bucketName := os.Getenv("BUCKET_NAME")
	if *bucketFlag != "" {
		bucketName = *bucketFlag
	}

	// Creates a Bucket instance.
	bucket := client.Bucket(bucketName)
	bucketPrefix := os.Getenv("BUCKET_PREFIX")
	if *prefixFlag != "" {
		bucketPrefix = *prefixFlag
	}

	excludeSubDirs := os.Getenv("EXCLUDE_SUBDIRS")
	if *excludeSubDirsFlag != "" {
		excludeSubDirs = *excludeSubDirsFlag
	}

	var s *slack.Client

	token := os.Getenv("SLACK_ACCESS_TOKEN")
	if token != "" {
		klog.Infof("setting up slack client (%d byte token)", len(token))
		s = slack.New(token)
	} else {
		klog.Infof("SLACK_ACCESS_TOKEN not set, won't actually post messages to Slack")
	}

	channel := os.Getenv("CHANNEL_ID")
	if *channelFlag != "" {
		channel = *channelFlag
	}
	if channel != "" {
		klog.Infof("Posting to channel ID=%s ...", channel)
	} else {
		klog.Infof("No channel ID provided, threaded replies may not work.")
	}

	var vtClient *vt.Client
	vtClient = nil
	if key := os.Getenv("VIRUSTOTAL_KEY"); key != "" {
		klog.Infof("Setting up VirusTotal client...")
		vtClient = vt.NewClient(key)
	}

	cc := &CollectConfig{Prefix: bucketPrefix, ExcludeSubdirs: strings.Split(excludeSubDirs, ","), Cutoff: cutoff}

	if *serveFlag {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		Serve(ctx, &Config{
			Bucket:            bucket,
			CollectConfig:     cc,
			Channel:           channel,
			SlackClient:       s,
			Addr:              fmt.Sprintf(":%s", port),
			MaxNoticesPerKind: *maxNoticesFlag,
			VirusTotalClient:  vtClient,
		})
	}

	rows := getRows(ctx, bucket, vtClient, cc)
	klog.Infof("collected %d rows", len(rows))
	notifier := NewNotifier()

	total := map[string]int{}

	for _, r := range rows {
		total[r.Kind]++
		if total[r.Kind] > *maxNoticesFlag {
			klog.Warningf("notification overflow for %s (%d), will not notify for: %s", r.Kind, total[r.Kind], r.Row)
			continue
		}
		if err := notifier.Notify(s, channel, r); err != nil {
			klog.Errorf("notify error: %v", err)
		}
	}

}
