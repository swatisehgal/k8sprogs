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
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	flag "github.com/spf13/pflag"

	podresources "k8s.io/kubernetes/pkg/kubelet/apis/podresources"
	podresourcesapi "k8s.io/kubernetes/pkg/kubelet/apis/podresources/v1alpha1"
	kubeletutil "k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	defaultPodResourcesPath    = "/var/lib/kubelet/pod-resources"
	defaultNamespace           = "default"
	defaultPodResourcesTimeout = 10 * time.Second
	defaultPodResourcesMaxSize = 1024 * 1024 * 16 // 16 Mb
	// obtained these values from node e2e tests : https://github.com/kubernetes/kubernetes/blob/82baa26905c94398a0d19e1b1ecf54eb8acb6029/test/e2e_node/util.go#L70
)

func main() {
	var err error
	autoReconnect := flag.BoolP("autoreconnect", "A", false, "don't give up if connection fails.")
	podResourcesSocketPath := flag.StringP("socket", "S", defaultPodResourcesPath, "podresources socket path.")
	listNamespace := flag.StringP("listnamespace", "N", defaultNamespace, "namespace to check")
	endpoint := flag.StringP("endpoint", "E", "list", "List/Watch/GetAvailableResources podresource API Endpoint")

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

	switch *endpoint {
	case "list":
		Listing(*autoReconnect, cli, *listNamespace)
	case "getavailableresources":
		GetAvailableResources(*autoReconnect, cli)
	}
}

func Listing(autoReconnect bool, cli podresourcesapi.PodResourcesListerClient, ns string) {
	resp, err := cli.List(context.TODO(), &podresourcesapi.ListPodResourcesRequest{})
	for {
		if err == nil {
			break
		} else {
			if !autoReconnect {
				log.Fatalf("failed to watch: %v", err)
			} else {
				log.Printf("Can't receive response: %v.Get(_) = _, %v", cli, err)
				time.Sleep(1 * time.Second)
			}
		}
	}
	showPodResources(resp, ns)
}

func showPodResources(resp *podresourcesapi.ListPodResourcesResponse, ns string) {
	for _, podResource := range resp.GetPodResources() {
		if podResource.GetNamespace() != ns {
			log.Printf("SKIP pod %q\n", podResource.Name)
			continue
		}
		for _, container := range podResource.GetContainers() {
			log.Printf("container %q\n", spew.Sdump(container))
		}

	}
}

func GetAvailableResources(autoReconnect bool, cli podresourcesapi.PodResourcesListerClient) {
	resp, err := cli.GetAvailableResources(context.TODO(), &podresourcesapi.AvailableResourcesRequest{})
	for {
		if err == nil {
			break
		} else {
			if !autoReconnect {
				log.Fatalf("failed to watch: %v", err)
			} else {
				log.Printf("Can't receive response: %v.Get(_) = _, %v", cli, err)
				time.Sleep(1 * time.Second)
			}
		}
	}
	showNodeResources(resp)
}

func showNodeResources(resp *podresourcesapi.AvailableResourcesResponse) {
	for _, device := range resp.GetDevices() {
			log.Printf("devices %q\n", spew.Sdump(device))
		}
	for _, cpuId := range resp.GetCpuIds() {
			log.Printf("cpuId %q\n", spew.Sdump(cpuId))
	}
}
