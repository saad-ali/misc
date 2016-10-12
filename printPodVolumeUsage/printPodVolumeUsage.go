/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
)

const (
	testProjectID = "saads-vms2"
)

func main() {
	// log.Println("Kubernetes volume discovery tool")
	// log.Println("  This tool assumes that kubectl command line tool is present and properly configured.")
	// log.Println("  It uses kubectl to discover all pods on the cluster that kubectl is pointing to.")
	// log.Println("  For each pod it will call kubectl describe to and compile a list of volume types used by the pod.")

	// Create a new PD
	podsJSON, _ := kubectlGetPods()
	printPodVolumes(podsJSON)
}

func printPodVolumes(podsJSON map[string]interface{}) map[string]uint {
	volumeCount := make(map[string]uint)
	for _, pod := range podsJSON["items"].([]interface{}) {
		//log.Println("POD:")
		for _, volume := range pod.(map[string]interface{})["spec"].(map[string]interface{})["volumes"].([]interface{}) {
			//log.Println("  Volume:")
			for key, _ := range volume.(map[string]interface{}) {
				if key != "name" {
					volumeCount[key]++
					// log.Printf(
					// 	"%v\r\n",
					// 	key,
					// )
				}
			}
		}
	}
	// log.Printf("Volumes used by this cluster: %v \r\n", volumeCount)
	for key, value := range volumeCount {
		fmt.Printf("%s: %v\r\n", key, value)
	}

	return volumeCount
}

func kubectlGetPods() (map[string]interface{}, error) {
	// log.Printf("Attempting to fetch complete list of pods\r\n")
	// defer fmt.Println("------------")

	cmdArgs := []string{
		"get",
		"pods",
		"--all-namespaces",
		"-o=json"}
	outputBytes, cmdErr := executeKubectlCmd(cmdArgs)
	if cmdErr != nil {
		log.Printf(
			"\"kubectl get pods\" failed with %v\r\n",
			cmdErr)
		return nil, cmdErr
	}

	parsedJson := make(map[string]interface{})
	if err := json.Unmarshal(outputBytes, &parsedJson); err != nil {
		return nil, err
	}

	// log.Printf(
	// 	"\"kubectl get pods\" succeeded. Output: %q\r\n",
	// 	parsedJson)
	return parsedJson, nil
}

func executeKubectlCmd(cmdArgs []string) ([]byte, error) {
	// log.Printf("Executing: kubectl %v\r\n", cmdArgs)
	command := exec.Command("kubectl", cmdArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf(
			"failed: err=%v\noutput: %s\n",
			err,
			string(output))
	}

	return output, nil
}
