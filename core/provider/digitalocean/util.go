package digitalocean

import (
	"fmt"
)

func convertEnvMapToList(env map[string]string) []string {
	envList := []string{}

	for key, value := range env {
		envList = append(envList, key+"="+value)
	}

	return envList
}

<<<<<<< HEAD
func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
=======
func formatUserData(commands []string) string {
	userData := `#cloud-config
runcmd:`

	for _, command := range commands {
		userData += fmt.Sprintf("\n- %s", command)
	}

	return userData
>>>>>>> 834565c (feat(digitalocean): add support for configuring telemetry)
}
