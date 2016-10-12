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

	"github.com/golang/glog"
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
	podsJSON, err := kubectlGetPods()
	if err != nil {
		glog.Fatalf("failed to get pods: %v", err)
	}
	printPodVolumes(podsJSON)
}

func printPodVolumes(podsJSON map[string]interface{}) map[string]uint {
	volumeCount := make(map[string]uint)

	items := podsJSON["items"]
	if items == nil {
		return volumeCount
	}

	for _, pod := range items.([]interface{}) {
		//log.Println("POD:")
		spec := pod.(map[string]interface{})["spec"]
		if spec == nil {
			continue
		}
		volumes := spec.(map[string]interface{})["volumes"]
		if volumes == nil {
			continue
		}
		for _, volume := range volumes.([]interface{}) {
			//log.Println("  Volume:")
			for key, value := range volume.(map[string]interface{}) {
				if key != "name" {
					if key != "persistentVolumeClaim" {
						// Not PVC count and move on
						volumeCount[key]++
						continue
					}

					// PVC must be dereferenced
					// fmt.Printf("PVC must be dereferenced\r\n")
					namespace := ""
					metadata := pod.(map[string]interface{})["metadata"]
					if metadata != nil {
						namespaceObj := metadata.(map[string]interface{})["namespace"]
						if namespaceObj != nil {
							namespace = namespaceObj.(string)
						}
					}

					claimNameObj := value.(map[string]interface{})["claimName"]
					if claimNameObj != nil {
						claimName := claimNameObj.(string)
						volumeType := dereferencePVC(namespace, claimName)
						volumeCount[volumeType]++
					}
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

func dereferencePVC(pvcNamespace, pvcName string) string {
	pvName, err := kubectlGetPVC(pvcNamespace, pvcName)
	if err != nil {
		log.Printf("failed to get PVC: %v", err)
		return "failedToDerefPVC"
	}
	// fmt.Printf("PVC %s/%s is bound to PV %s\r\n", pvcNamespace, pvcName, pvName)
	volumeType, getPVErr := kubectlGetPV(pvName)
	if getPVErr != nil {
		log.Printf("failed to get PV: %v", err)
		return "failedToDerefPV"
	}

	return volumeType
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

func kubectlGetPVC(namespace, name string) (string, error) {
	// log.Printf("Attempting to fetch PVC namespace: %q name: %q\r\n", namespace, name)
	// defer fmt.Println("------------")

	if namespace == "" {
		namespace = "default"
	}

	cmdArgs := []string{
		"get",
		"pvc",
		name,
		"--namespace=" + namespace,
		"-o=json"}
	outputBytes, cmdErr := executeKubectlCmd(cmdArgs)
	if cmdErr != nil {
		log.Printf(
			"\"kubectl get pvc %s --namespace=%s -o=json\" failed with %v\r\n",
			name,
			namespace,
			cmdErr)
		return "", cmdErr
	}

	parsedJson := make(map[string]interface{})
	if err := json.Unmarshal(outputBytes, &parsedJson); err != nil {
		return "", err
	}

	// log.Printf(
	// 	"\"kubectl get pods\" succeeded. Output: %q\r\n",
	// 	parsedJson)
	// fmt.Printf("pvc: %q\r\n", parsedJson)

	status := parsedJson["status"]
	if status == nil {
		return "", fmt.Errorf("Error PVC namespace: %q name: %q parsed JSON does not contain status\r\n", namespace, name)
	}

	phaseObj := status.(map[string]interface{})["phase"]
	if phaseObj == nil {
		return "", fmt.Errorf("Error PVC namespace: %q name: %q parsed JSON does not contain status.phase\r\n", namespace, name)
	}

	phase := phaseObj.(string)
	if phase != "Bound" {
		return "unboundPVC", nil
	}

	spec := parsedJson["spec"]
	if spec == nil {
		return "", fmt.Errorf("Error PVC namespace: %q name: %q parsed JSON does not contain spec\r\n", namespace, name)
	}

	pvNameObj := spec.(map[string]interface{})["volumeName"]
	if pvNameObj == nil {
		return "", fmt.Errorf("Error PVC namespace: %q name: %q parsed JSON does not contain spec.volumeName\r\n", namespace, name)
	}

	pvName := pvNameObj.(string)
	return pvName, nil
}

func kubectlGetPV(name string) (string, error) {
	// log.Printf("Attempting to fetch PV name: %q\r\n", name)
	// defer fmt.Println("------------")

	cmdArgs := []string{
		"get",
		"pv",
		name,
		"-o=json"}
	outputBytes, cmdErr := executeKubectlCmd(cmdArgs)
	if cmdErr != nil {
		log.Printf(
			"\"kubectl get pvc %s -o=json\" failed with %v\r\n",
			name,
			cmdErr)
		return "", cmdErr
	}

	parsedJson := make(map[string]interface{})
	if err := json.Unmarshal(outputBytes, &parsedJson); err != nil {
		return "", err
	}

	spec := parsedJson["spec"]
	if spec == nil {
		return "", fmt.Errorf("Error PV name: %q parsed JSON does not contain spec\r\n", name)
	}

	for key, _ := range spec.(map[string]interface{}) {
		if key == "capacity" {
			continue
		}
		if key == "accessModes" {
			continue
		}
		if key == "claimRef" {
			continue
		}
		if key == "persistentVolumeReclaimPolicy" {
			continue
		}
		return key, nil
	}

	return "", fmt.Errorf("Error PV name: %q parsed JSON does not contain a volume type: %v\r\n", parsedJson)
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
