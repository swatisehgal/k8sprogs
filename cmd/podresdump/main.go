/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2020 Red Hat, Inc.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	flag "github.com/spf13/pflag"

	podresources "k8s.io/kubernetes/pkg/kubelet/apis/podresources"
	podresourcesapi "k8s.io/kubernetes/pkg/kubelet/apis/podresources/v1alpha1"
	kubeletutil "k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	defaultPodResourcesPath    = "/var/lib/kubelet/pod-resources"
	defaultPodResourcesTimeout = 10 * time.Second
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	// obtained these values from node e2e tests : https://github.com/kubernetes/kubernetes/blob/82baa26905c94398a0d19e1b1ecf54eb8acb6029/test/e2e_node/util.go#L70
)

func podActionToString(action podresourcesapi.WatchPodAction) string {
	switch action {
	case podresourcesapi.WatchPodAction_ADDED:
		return "ADD"
	case podresourcesapi.WatchPodAction_MODIFIED:
		return "MOD"
	case podresourcesapi.WatchPodAction_DELETED:
		return "DEL"
	default:
		return "???"
	}
}

func main() {
	var err error
	autoReconnect := flag.BoolP("autoreconnect", "A", false, "don't give up if connection fails.")
	podResourcesSocketPath := flag.StringP("socket", "S", defaultPodResourcesPath, "podresources socket path.")
	flag.Parse()

	sockPath, err := kubeletutil.LocalEndpoint(*podResourcesSocketPath, podresources.Socket)
	if err != nil {
		log.Fatalf("%s", err)
	}

	cli, conn, err := podresources.GetClient(sockPath, defaultPodResourcesTimeout, defaultPodResourcesMaxSize)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	var watcher podresourcesapi.PodResourcesLister_WatchClient
	for {
		watcher, err = cli.Watch(context.TODO(), &podresourcesapi.WatchPodResourcesRequest{})
		if err == nil {
			break
		} else {
			if !*autoReconnect {
				log.Fatalf("failed to watch: %v", err)
			} else {
				log.Printf("error watching: %v", err)
				time.Sleep(1 * time.Second)
			}
		}

	}

	respsCh := make(chan *podresourcesapi.WatchPodResourcesResponse)
	stopCh := make(chan bool, 1)
	sigsCh := make(chan os.Signal, 1)
	signal.Notify(sigsCh, os.Interrupt)

	started := time.Now()

	go func() {
		for {
			resp, err := watcher.Recv()
			if err != nil {
				log.Printf("%s", err)
				stopCh <- true
				break
			}
			respsCh <- resp
		}
	}()

	var messages uint64

	done := false
	for !done {
		select {
		case <-stopCh:
			done = true
		case <-sigsCh:
			done = true
		case resp := <-respsCh:
			jsonBytes, err := json.Marshal(resp.PodResources)
			if err != nil {
				log.Printf("%v", err)
			} else {
				fmt.Printf("%s %s\n", podActionToString(resp.Action), string(jsonBytes))
				messages++
			}
		}
	}

	log.Printf("%v messages in %v", messages, time.Now().Sub(started))
}
