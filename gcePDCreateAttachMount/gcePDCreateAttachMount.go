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
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"path"
	"strings"
	"time"
)

const (
	testProjectID        = "saads-vms2"
	testProjectZone      = "us-central1-b"
	testInstance0Name    = "e2e-test-saadali-minion-group-s71i"
	testInstance1Name    = "e2e-test-saadali-minion-group-68jg"
	diskByIdPath         = "/dev/disk/by-id/"
	diskGooglePrefix     = "google-"
	diskScsiGooglePrefix = "scsi-0Google_PersistentDisk_"
	testFSType           = "ext4"
	globalMountPath      = "/var/lib/saad/plugins/kubernetes.io/gce-pd/mounts/"
	finalMountPath       = "/var/lib/saad/pods/volumes/kubernetes.io~gce-pd/"
)

func main() {
	// Create a new PD
	t := time.Now()
	generatedPdName := fmt.Sprintf("test-%s", t.Format("20060102150405"))
	pdName, createErr := createPDWithRetry(generatedPdName)
	if createErr != nil {
		log.Fatalln(createErr)
	}

	devPath := getPDDevPath(pdName)
	devGlobalMountPath := getDeviceGlobalMountPath(pdName)
	finalMountPath := getFinalMountPath(pdName)
	fatalError := false

	// Attach PD RW to host0
	log.Println("***Attach PD RW to host0")
	attachErr := attachDiskWithRetry(pdName, testInstance0Name, false /* readonly */)
	if attachErr != nil {
		log.Fatalln(attachErr)
		fatalError = true
	}

	// ls disks on host0
	o, _ := executeRemoteGCloudCmd("ls /dev/disk/by-id/", testInstance0Name)
	log.Printf("ls /dev/disk/by-id/\r\n%v", string(o))

	// Format attached disk and mount PD to global mount point on host0
	mountDevErr := mountDevice(devPath, devGlobalMountPath, testInstance0Name, testFSType, false /* readOnly */)
	if mountDevErr != nil {
		log.Println(mountDevErr)
		fatalError = true
	}

	// Bind Mount global mount point to final mount point on host0
	bindMountErr := bindMountToFinalPath(devGlobalMountPath, finalMountPath, testInstance0Name, false /* readOnly */)
	if bindMountErr != nil {
		log.Println(bindMountErr)
		fatalError = true
	}

	fileContent := "hello world"
	fileName := "mytest.log"
	_, writeFileErr := WriteContentToFile(fileContent, path.Join(finalMountPath, fileName), testInstance0Name)
	if writeFileErr != nil {
		log.Println(writeFileErr)
		fatalError = true
	}

	readFileContent, readFileErr := ReadContentsFromFile(path.Join(finalMountPath, fileName), testInstance0Name)
	if readFileErr != nil {
		log.Println(readFileErr)
		fatalError = true
	}

	if fileContent != readFileContent {
		log.Printf("Read file content differs. Expected: <%s> Actual: <%s>", fileContent, readFileContent)
		fatalError = true
	}

	log.Println("Sleeping before unmount")
	time.Sleep(3 * time.Second)

	// Unmount bind mount on host 0
	removeBindMountErr := removeBindMount(finalMountPath, testInstance0Name)
	if removeBindMountErr != nil {
		log.Println(removeBindMountErr)
	}

	// Unmount PD from global mount point on host 0
	unmountDevErr := unmountDevice(devGlobalMountPath, testInstance0Name)
	if unmountDevErr != nil {
		log.Println(unmountDevErr)
	}

	// Detach PD from host0
	detachErr := detachDiskWithRetry(pdName, testInstance0Name)
	if detachErr != nil {
		log.Println(detachErr)
		fatalError = true
	}

	log.Println("***Detached PD RW from host0")

	// Attach PD RW to host1
	log.Println("***Attaching PD RW to host1")
	attachErr = attachDiskWithRetry(pdName, testInstance1Name, false /* readonly */)
	if attachErr != nil {
		log.Println(attachErr)
		fatalError = true
	}

	// Mount PD RW to global mount point on host1
	mountDevErr = mountDevice(devPath, devGlobalMountPath, testInstance1Name, testFSType, false /* readOnly */)
	if mountDevErr != nil {
		log.Println(mountDevErr)
		fatalError = true
	}

	// Bind Mount RW global mount point to final mount point on host1
	bindMountErr = bindMountToFinalPath(devGlobalMountPath, finalMountPath, testInstance1Name, false /* readOnly */)
	if bindMountErr != nil {
		log.Println(bindMountErr)
		fatalError = true
	}

	readFileContent, readFileErr = ReadContentsFromFile(path.Join(finalMountPath, fileName), testInstance1Name)
	if readFileErr != nil {
		log.Println(readFileErr)
		fatalError = true
	}

	if fileContent != readFileContent {
		log.Printf("Read file content differs. Expected: <%s> Actual: <%s>", fileContent, readFileContent)
		fatalError = true
	}

	log.Println("Sleeping before unmount")
	time.Sleep(10 * time.Second)

	// Unmount RW bind mount on host1
	removeBindMountErr = removeBindMount(finalMountPath, testInstance1Name)
	if removeBindMountErr != nil {
		log.Println(removeBindMountErr)
	}

	// Unmount PD from global mount point on host1
	unmountDevErr = unmountDevice(devGlobalMountPath, testInstance1Name)
	if unmountDevErr != nil {
		log.Println(unmountDevErr)
	}

	// Detach PD RW to host1
	detachErr = detachDiskWithRetry(pdName, testInstance1Name)
	if detachErr != nil {
		log.Println(detachErr)
		fatalError = true
	}
	log.Println("***Detached PD RW from host1")

	// // Attach PD RO to host0
	// log.Println("***Attaching PD RO to host0")
	// attachErr = attachDiskWithRetry(pdName, testInstance0Name, true /* readonly */)
	// if attachErr != nil {
	// 	log.Println(attachErr)
	// 	fatalError = true
	// }

	// // Mount PD RO to global mount point on host0
	// mountDevErr = mountDevice(devPath, devGlobalMountPath, testInstance0Name, testFSType, true /* readOnly */)
	// if mountDevErr != nil {
	// 	log.Println(mountDevErr)
	// 	fatalError = true
	// }

	// // Bind Mount RO global mount point to final mount point on host0
	// bindMountErr = bindMountToFinalPath(devGlobalMountPath, finalMountPath, testInstance0Name, true /* readOnly */)
	// if bindMountErr != nil {
	// 	log.Println(bindMountErr)
	// 	fatalError = true
	// }

	// // Attach PD RO to host1
	// log.Println("***Attaching PD RO to host1")
	// attachErr = attachDiskWithRetry(pdName, testInstance1Name, true /* readonly */)
	// if attachErr != nil {
	// 	log.Println(attachErr)
	// 	fatalError = true
	// }

	// // Mount PD RO to global mount point on host1
	// mountDevErr = mountDevice(devPath, devGlobalMountPath, testInstance1Name, testFSType, true /* readOnly */)
	// if mountDevErr != nil {
	// 	log.Println(mountDevErr)
	// 	fatalError = true
	// }

	// // Bind Mount RO global mount point to final mount point on host1
	// bindMountErr = bindMountToFinalPath(devGlobalMountPath, finalMountPath, testInstance1Name, true /* readOnly */)
	// if bindMountErr != nil {
	// 	log.Println(bindMountErr)
	// 	fatalError = true
	// }

	// // Unmount RO bind mount on host0
	// removeBindMountErr = removeBindMount(finalMountPath, testInstance0Name)
	// if removeBindMountErr != nil {
	// 	log.Println(removeBindMountErr)
	// }

	// // Unmount PD from global mount point on host0
	// unmountDevErr = unmountDevice(devGlobalMountPath, testInstance0Name)
	// if unmountDevErr != nil {
	// 	log.Println(unmountDevErr)
	// }

	// // Detach PD RO to host0
	// detachErr = detachDiskWithRetry(pdName, testInstance0Name)
	// if detachErr != nil {
	// 	log.Println(detachErr)
	// 	fatalError = true
	// }
	// log.Println("***Detached PD RO to host0")

	// // Unmount RO bind mount on host1
	// removeBindMountErr = removeBindMount(finalMountPath, testInstance1Name)
	// if removeBindMountErr != nil {
	// 	log.Println(removeBindMountErr)
	// }

	// // Unmount PD from global mount point on host1
	// unmountDevErr = unmountDevice(devGlobalMountPath, testInstance1Name)
	// if unmountDevErr != nil {
	// 	log.Println(unmountDevErr)
	// }

	// // Detach PD RO to host1
	// detachErr = detachDiskWithRetry(pdName, testInstance1Name)
	// if detachErr != nil {
	// 	log.Println(detachErr)
	// 	fatalError = true
	// }
	// log.Println("***Detached PD RO to host1")

	// Delete PD on completion
	deleteErr := deletePDWithRetry(pdName)
	if deleteErr != nil {
		log.Println(deleteErr)
		fatalError = true
	}

	if fatalError {
		log.Fatalf("Fatal error\r\n")
	}
}

func createPDWithRetry(pdName string) (string, error) {
	newDiskName := ""
	var err error
	for start := time.Now(); time.Since(start) < 180*time.Second; time.Sleep(5 * time.Second) {
		if newDiskName, err = createPD(pdName); err != nil {
			log.Printf("Couldn't create a new PD. Sleeping 5 seconds (%v)\r\n", err)
			continue
		}
		log.Printf("Successfully created a new PD: %q.\r\n", newDiskName)
		break
	}
	return newDiskName, err
}

func createPD(pdName string) (string, error) {
	log.Printf("Attempting to create PD %q\r\n", pdName)
	defer fmt.Println("------------")

	cmdArgs := []string{
		"compute",
		"--quiet",
		"--project=" + testProjectID,
		"disks",
		"create",
		"--zone=" + testProjectZone,
		"--size=10GB",
		pdName}
	outputBytes, cmdErr := executeGCloudCmd(cmdArgs)
	if cmdErr != nil {
		log.Printf(
			"Creating PD %q failed with %v\r\n",
			pdName,
			cmdErr)
		return "", cmdErr
	}

	log.Printf(
		"Created PD %q successfully. Output: %q\r\n",
		pdName,
		string(outputBytes))
	return pdName, nil
}

func deletePDWithRetry(pdName string) error {
	var err error
	for start := time.Now(); time.Since(start) < 180*time.Second; time.Sleep(5 * time.Second) {
		if err = deletePD(pdName); err != nil {
			log.Printf("Couldn't delete PD %q. Sleeping 5 seconds (%v)\r\n", pdName, err)
			continue
		}
		log.Printf("Deleted PD %v", pdName)
		break
	}
	return err
}

func deletePD(pdName string) error {
	log.Printf("Attempting to delete PD %q\r\n", pdName)
	defer fmt.Println("------------")

	cmdArgs := []string{
		"compute",
		"--quiet",
		"--project=" + testProjectID,
		"disks",
		"delete",
		"--zone=" + testProjectZone,
		pdName}
	outputBytes, cmdErr := executeGCloudCmd(cmdArgs)
	if cmdErr != nil {
		log.Printf(
			"Deleting PD %q failed with %v\r\n",
			pdName,
			cmdErr)
		return cmdErr
	}

	log.Printf(
		"Deleting PD %q succeeded. Output: %q\r\n",
		pdName,
		string(outputBytes))
	return nil
}

func attachDiskWithRetry(pdName, instanceName string, readonly bool) error {
	var err error
	for start := time.Now(); time.Since(start) < 180*time.Second; time.Sleep(5 * time.Second) {
		if err = attachDisk(pdName, instanceName, readonly); err != nil {
			log.Printf("Couldn't attach PD %q to %q. Sleeping 5 seconds (%v)\r\n", pdName, instanceName, err)
			continue
		}
		log.Printf("Successfully attach PD %q to %q.\r\n", pdName, instanceName)
		break
	}
	return err
}

func attachDisk(pdName, instanceName string, readonly bool) error {
	mode := "ro"
	if !readonly {
		mode = "rw"
	}

	log.Printf("Attempting to attach PD %q to %q as %q\r\n", pdName, instanceName, mode)
	defer fmt.Println("------------")

	cmdArgs := []string{
		"compute",
		"instances",
		"--quiet",
		"attach-disk",
		instanceName,
		"--disk=" + pdName,
		"--device-name=" + pdName,
		"--mode=" + mode,
		"--zone=" + testProjectZone}
	outputBytes, cmdErr := executeGCloudCmd(cmdArgs)
	if cmdErr != nil {
		log.Printf(
			"Attaching PD %q to %q as %q failed with %v\r\n",
			pdName,
			instanceName,
			mode,
			cmdErr)
		return cmdErr
	}

	log.Printf(
		"Attaching PD %q to %q as %q succeeded. Output: %q\r\n",
		pdName,
		instanceName,
		mode,
		string(outputBytes))
	return nil
}

func detachDiskWithRetry(pdName, instanceName string) error {
	var err error
	for start := time.Now(); time.Since(start) < 180*time.Second; time.Sleep(5 * time.Second) {
		if err = detachDisk(pdName, instanceName); err != nil {
			log.Printf("Couldn't detach PD %q to %q. Sleeping 5 seconds (%v)\r\n", pdName, instanceName, err)
			continue
		}
		log.Printf("Successfully detach PD %q to %q.\r\n", pdName, instanceName)
		break
	}
	return err
}

func detachDisk(pdName, instanceName string) error {
	log.Printf("Attempting to detach PD %q from %q\r\n", pdName, instanceName)
	defer fmt.Println("------------")

	cmdArgs := []string{
		"compute",
		"instances",
		"--quiet",
		"detach-disk",
		instanceName,
		"--disk=" + pdName,
		"--zone=" + testProjectZone}
	outputBytes, cmdErr := executeGCloudCmd(cmdArgs)
	if cmdErr != nil {
		log.Printf(
			"Detaching PD %q from %q failed with %v\r\n",
			pdName,
			instanceName,
			cmdErr)
		return cmdErr
	}

	log.Printf(
		"Detaching PD %q from %q succeeded. Output: %q\r\n",
		pdName,
		instanceName,
		string(outputBytes))
	return nil
}

func bindMountToFinalPath(deviceMountPath, finalMountPath, instanceName string, readOnly bool) error {
	if _, err := runMkDir(finalMountPath, instanceName); err != nil {
		return err
	}

	// Perform a bind mount to the full path to allow duplicate mounts of the same PD.
	options := []string{"bind"}
	if readOnly {
		options = append(options, "ro")
	}

	if _, err := mount(deviceMountPath, finalMountPath, instanceName, "" /* fstype */, options); err != nil {
		unmount(finalMountPath, instanceName)
		runRmDir(finalMountPath, instanceName)
		return err
	}
	log.Printf("Successfully bind mounted %q to %q\r\n", finalMountPath, deviceMountPath)
	return nil
}

func removeBindMount(finalMountPath, instanceName string) error {
	_, err := unmount(finalMountPath, instanceName)
	runRmDir(finalMountPath, instanceName)
	if err == nil {
		log.Printf("Successfully removed bind mount %q\r\n", finalMountPath)
	}
	return err
}

func mountDevice(devicePath, deviceMountPath, instanceName, fstype string, readOnly bool) error {
	if _, err := runMkDir(deviceMountPath, instanceName); err != nil {
		return err
	}

	options := []string{}
	if readOnly {
		options = append(options, "ro")
	}

	if _, err := formatAndMount(devicePath, deviceMountPath, instanceName, fstype, options); err != nil {
		runRmDir(deviceMountPath, instanceName)
		return err
	}
	log.Printf("Successfully mounted %q to %q\r\n", deviceMountPath, devicePath)
	return nil
}

func unmountDevice(mountPath, instanceName string) error {
	_, err := unmount(mountPath, instanceName)
	runRmDir(mountPath, instanceName)
	if err == nil {
		log.Printf("Successfully unmounted %q\r\n", mountPath)
	}
	return err
}

func formatAndMount(devPath, mountPath, instanceName, fstype string, options []string) ([]byte, error) {
	// Don't attempt to format if mounting as readonly. Go straight to mounting.
	for _, option := range options {
		if option == "ro" {
			_, err := mount(devPath, mountPath, instanceName, fstype, options)
			if err == nil {
				log.Printf("Successfully mounted %q to %q\r\n", mountPath, devPath)
			}
			return nil, err
		}
	}

	options = append(options, "defaults")

	// Run fsck on the disk to fix repairable issues
	outputBytes, err := runFsck(devPath, instanceName)
	if err != nil {
		if strings.Contains(err.Error(), "exist status 1") {
			// exit status 1 -- 'fsck' found errors and corrected them
			log.Printf("Device %s has errors which were corrected by fsck.", devPath)
		} else if strings.Contains(err.Error(), "exist status 4") {
			// exit status 4 -- 'fsck' found errors but exited without correcting them
			log.Printf("'fsck' found errors on device %s but could not correct them: %s.", devPath, string(outputBytes))
			return outputBytes, err
		} else {
			log.Printf("`fsck` error %s", err)
		}
	}

	// Try to mount the disk
	_, err = mount(devPath, mountPath, instanceName, fstype, options)
	if err != nil {
		// It is possible that this disk is not formatted. Double check using diskLooksUnformatted
		notFormatted, err := diskLooksUnformatted(devPath, instanceName)
		if err == nil && notFormatted {
			log.Printf("Disk looks unformated, will attempt to format it.")
			_, err := format(devPath, instanceName, fstype)
			if err == nil {
				// the disk has been formatted successfully try to mount it again.
				log.Printf("Disk formated successfully, will attempt to mount it.")
				_, err = mount(devPath, mountPath, instanceName, fstype, options)
				if err == nil {
					log.Printf("Successfully formatAndMount %q to %q\r\n", mountPath, devPath)
				}
				return nil, err
			}
			return nil, err
		}
	}

	return nil, err
}

func format(devPath, instanceName string, fstype string) ([]byte, error) {
	log.Printf("Attempting to format %q on %q with fstype %q and options %v\r\n", devPath, instanceName, fstype)
	defer fmt.Println("------------")

	formatCmd := "mkfs." + fstype + " " + devPath
	if len(fstype) == 0 {
		fstype = "ext4"
	}

	if fstype == "ext4" || fstype == "ext3" {
		formatCmd = "mkfs." + fstype + " -E lazy_itable_init=0,lazy_journal_init=0 -F " + devPath
	}

	outputBytes, cmdErr := executeRemoteGCloudCmd(formatCmd, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed to format %q on %q with fstype %q and options %v. error: %v\r\n",
			devPath,
			instanceName,
			fstype,
			cmdErr)

		return outputBytes, cmdErr
	}

	return outputBytes, nil
}

func mount(devPath, mountPath, instanceName string, fstype string, options []string) ([]byte, error) {
	bind, bindRemountOpts := isBind(options)

	if bind {
		outputBytes, err := doMount(devPath, mountPath, instanceName, fstype, []string{"bind"})
		if err != nil {
			return outputBytes, err
		}
		return doMount(devPath, mountPath, instanceName, fstype, bindRemountOpts)
	}

	return doMount(devPath, mountPath, instanceName, fstype, options)
}

func unmount(mountPath, instanceName string) ([]byte, error) {
	log.Printf("Attempting to unmount %q on %q \r\n", mountPath, instanceName)
	defer fmt.Println("------------")

	unmountCmd := "umount " + mountPath
	outputBytes, cmdErr := executeRemoteGCloudCmd(unmountCmd, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed to unmount %q on %q. error: %v\r\n",
			mountPath,
			instanceName,
			cmdErr)

		return outputBytes, cmdErr
	}

	return outputBytes, nil
}

func isBind(options []string) (bool, []string) {
	bindRemountOpts := []string{"remount"}
	bind := false

	if len(options) != 0 {
		for _, option := range options {
			switch option {
			case "bind":
				bind = true
				break
			case "remount":
				break
			default:
				bindRemountOpts = append(bindRemountOpts, option)
			}
		}
	}

	return bind, bindRemountOpts
}

func runMkDir(dir, instanceName string) ([]byte, error) {
	log.Printf("Attempting to create directory %q on %q \r\n", dir, instanceName)
	defer fmt.Println("------------")

	mkdirCmd := "mkdir -p -m 0750 " + dir
	outputBytes, cmdErr := executeRemoteGCloudCmd(mkdirCmd, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed to create directory %q on %q. error: %v\r\n",
			dir,
			instanceName,
			cmdErr)

		return outputBytes, cmdErr
	}

	return outputBytes, nil
}

func runRmDir(dir, instanceName string) ([]byte, error) {
	log.Printf("Attempting to remove directory %q on %q \r\n", dir, instanceName)
	defer fmt.Println("------------")

	rmdirCmd := "rmdir " + dir
	outputBytes, cmdErr := executeRemoteGCloudCmd(rmdirCmd, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed to remove directory %q on %q. error: %v\r\n",
			dir,
			instanceName,
			cmdErr)

		return outputBytes, cmdErr
	}

	return outputBytes, nil
}

func doMount(devPath, mountPath, instanceName string, fstype string, options []string) ([]byte, error) {
	log.Printf("Attempting to mount %q to %q on %q with fstype %q and options %v\r\n", mountPath, devPath, instanceName, fstype, options)
	defer fmt.Println("------------")

	mountCmd := makeMountCmd(devPath, mountPath, fstype, options)
	outputBytes, cmdErr := executeRemoteGCloudCmd(mountCmd, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed mount %q to %q on %q with fstype %q and options %v. error: %v\r\n",
			mountPath,
			devPath,
			instanceName,
			fstype,
			options,
			cmdErr)

		return outputBytes, cmdErr
	}

	return outputBytes, nil
}

func makeMountCmd(devPath, mountPath, fstype string, options []string) string {
	// Build mount command as follows:
	//   mount [-t $fstype] [-o $options] [$source] $target
	var mountCmdBytes bytes.Buffer
	mountCmdBytes.WriteString("mount")
	if len(fstype) > 0 {
		mountCmdBytes.WriteString(" -t ")
		mountCmdBytes.WriteString(fstype)
	}
	if len(options) > 0 {
		mountCmdBytes.WriteString(" -o ")
		mountCmdBytes.WriteString(strings.Join(options, ","))
	}
	if len(devPath) > 0 {
		mountCmdBytes.WriteString(" ")
		mountCmdBytes.WriteString(devPath)
	}
	mountCmdBytes.WriteString(" ")
	mountCmdBytes.WriteString(mountPath)

	return mountCmdBytes.String()
}

const (
	// 'fsck' found errors and corrected them
	fsckErrorsCorrected = 1
	// 'fsck' found errors but exited without correcting them
	fsckErrorsUncorrected = 4
)

func runFsck(devPath, instanceName string) ([]byte, error) {
	log.Printf("Run fsck on disk %q on %q to fix repairable issues\r\n", devPath, instanceName)
	defer fmt.Println("------------")

	remoteCommand := "fsck -a " + devPath
	outputBytes, cmdErr := executeRemoteGCloudCmd(remoteCommand, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed running fsck on disk %q on %q to fix repairable issues. error: %v\r\n",
			devPath,
			instanceName,
			cmdErr)

		return outputBytes, cmdErr
	}

	return outputBytes, nil
}

func WriteContentToFile(fileContents, filePath, instanceName string) ([]byte, error) {
	log.Printf("Writing %q to %q on %q\r\n", fileContents, filePath, instanceName)
	defer fmt.Println("------------")

	remoteCommand := fmt.Sprintf("echo '%s' > '%s'; sync", fileContents, filePath)
	outputBytes, cmdErr := executeRemoteGCloudCmd(remoteCommand, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed writing %q to %q on %q. error: %v\r\n",
			fileContents,
			filePath,
			instanceName,
			cmdErr)

		return outputBytes, cmdErr
	}

	return outputBytes, nil
}

func ReadContentsFromFile(filePath, instanceName string) (string, error) {
	log.Printf("Reading %q on %q\r\n", filePath, instanceName)
	defer fmt.Println("------------")

	remoteCommand := fmt.Sprintf("cat '%s'", filePath)
	outputBytes, cmdErr := executeRemoteGCloudCmd(remoteCommand, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Reading %q on %q. error: %v\r\n",
			filePath,
			instanceName,
			cmdErr)

		return strings.TrimSpace(string(outputBytes)), cmdErr
	}

	return strings.TrimSpace(string(outputBytes)), nil
}

func diskLooksUnformatted(devPath, instanceName string) (bool, error) {
	log.Printf("Checking if %q is formatted on %q\r\n", devPath, instanceName)
	defer fmt.Println("------------")

	remoteCommand := "lsblk -nd -o FSTYPE " + devPath
	outputBytes, cmdErr := executeRemoteGCloudCmd(remoteCommand, instanceName)
	if cmdErr != nil {
		log.Printf(
			"Failed checking if %q is formatted on %q with %v\r\n",
			devPath,
			instanceName,
			cmdErr)
		return false, cmdErr
	}

	output := strings.TrimSpace(string(outputBytes))
	result := output == ""
	log.Printf(
		"Checking if %q is formatted on %q. Result: %v Output: %q\r\n",
		devPath,
		instanceName,
		result,
		output)

	return result, nil
}

func executeRemoteGCloudCmd(remoteCommand, instanceName string) ([]byte, error) {
	cmdArgs := []string{
		"compute",
		"ssh",
		"root@" + instanceName,
		"--command",
		remoteCommand}
	return executeGCloudCmd(cmdArgs)
}

func executeGCloudCmd(cmdArgs []string) ([]byte, error) {
	log.Printf("Executing: gcloud %v\r\n", cmdArgs)
	command := exec.Command("gcloud", cmdArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf(
			"failed: err=%v\noutput: %s\n",
			err,
			string(output))
	}

	return output, nil
}

func getPDDevPath(pdName string) string {
	return path.Join(diskByIdPath, diskScsiGooglePrefix+pdName)
}

func getDeviceGlobalMountPath(pdName string) string {
	return path.Join(globalMountPath, pdName)
}

func getFinalMountPath(pdName string) string {
	return path.Join(finalMountPath, pdName)
}
